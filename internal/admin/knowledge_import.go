package admin

import (
	"fmt"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

const defaultKnowledgeImportMaxSourceBytes int64 = 1 << 20

type KnowledgeImportSourceType string

const (
	KnowledgeImportSourceLocalTextFile   KnowledgeImportSourceType = "local_text_file"
	KnowledgeImportSourceURLTextSnapshot KnowledgeImportSourceType = "url_text_snapshot"
)

type KnowledgeImportStatus string

const (
	KnowledgeImportPending   KnowledgeImportStatus = "pending"
	KnowledgeImportRunning   KnowledgeImportStatus = "running"
	KnowledgeImportCompleted KnowledgeImportStatus = "completed"
	KnowledgeImportFailed    KnowledgeImportStatus = "failed"
	KnowledgeImportPartial   KnowledgeImportStatus = "partial"
)

const (
	KnowledgeImportSkipDuplicateContent    = "duplicate_content"
	KnowledgeImportWarningInstructionLikeText = "instruction_like_text"
)

type KnowledgeImportRequest struct {
	SpaceID     string                    `json:"space_id,omitempty"`
	SourceType  KnowledgeImportSourceType `json:"source_type"`
	SourceLabel string                    `json:"source_label,omitempty"`
	Sources     []KnowledgeImportSource   `json:"sources"`
}

type KnowledgeImportSource struct {
	Name      string            `json:"name,omitempty"`
	URI       string            `json:"uri,omitempty"`
	Content   string            `json:"content"`
	SizeBytes int64             `json:"size_bytes,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type KnowledgeImportSkippedSource struct {
	Name               string `json:"name"`
	Reason             string `json:"reason"`
	ExistingDocumentID string `json:"existing_document_id,omitempty"`
}

type KnowledgeImportFailedSource struct {
	Name   string `json:"name,omitempty"`
	URI    string `json:"uri,omitempty"`
	Reason string `json:"reason"`
}

type KnowledgeImportJob struct {
	ID                  string                         `json:"id"`
	TenantID            string                         `json:"tenant_id"`
	SpaceID             string                         `json:"space_id"`
	SourceType          KnowledgeImportSourceType      `json:"source_type"`
	SourceLabel         string                         `json:"source_label,omitempty"`
	Status              KnowledgeImportStatus          `json:"status"`
	SourceCount         int                            `json:"source_count"`
	ImportedDocumentIDs []string                       `json:"imported_document_ids,omitempty"`
	SkippedSources      []KnowledgeImportSkippedSource `json:"skipped_sources,omitempty"`
	FailedSources       []KnowledgeImportFailedSource  `json:"failed_sources,omitempty"`
	FailureCode         string                         `json:"failure_code,omitempty"`
	FailureCause        string                         `json:"failure_cause,omitempty"`
	Metadata            map[string]string              `json:"metadata,omitempty"`
	CreatedAt           time.Time                      `json:"created_at"`
	UpdatedAt           time.Time                      `json:"updated_at"`
}

type KnowledgeImportService struct {
	store          KnowledgeStore
	knowledge      KnowledgeService
	now            func() time.Time
	maxSourceBytes int64
}

func NewKnowledgeImportService(store KnowledgeStore, knowledge KnowledgeService) KnowledgeImportService {
	return KnowledgeImportService{
		store:          store,
		knowledge:      knowledge,
		now:            func() time.Time { return time.Now().UTC() },
		maxSourceBytes: defaultKnowledgeImportMaxSourceBytes,
	}
}

func (s KnowledgeImportService) List(tenantID, spaceID string) ([]KnowledgeImportJob, error) {
	return s.store.ListKnowledgeImportJobs(tenantID, spaceID)
}

func (s KnowledgeImportService) Import(tenantID string, request KnowledgeImportRequest) (KnowledgeImportJob, error) {
	space, err := s.knowledge.requireWritableSpace(tenantID, request.SpaceID)
	if err != nil {
		return KnowledgeImportJob{}, err
	}
	now := s.now()
	job := KnowledgeImportJob{
		ID:          fmt.Sprintf("import-%d", now.UnixNano()),
		TenantID:    tenantID,
		SpaceID:     space.ID,
		SourceType:  request.SourceType,
		SourceLabel: strings.TrimSpace(request.SourceLabel),
		Status:      KnowledgeImportRunning,
		SourceCount: len(request.Sources),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if len(request.Sources) == 0 {
		return s.failJob(job, "empty_sources", "knowledge import requires at least one source")
	}
	if _, err := s.store.SaveKnowledgeImportJob(job); err != nil {
		return KnowledgeImportJob{}, err
	}
	if !isSupportedKnowledgeImportSourceType(request.SourceType) {
		return s.failJob(job, "unsupported_source_type", "unsupported source type")
	}

	documents, err := s.knowledge.ListBySpace(tenantID, space.ID)
	if err != nil {
		return s.failJob(job, "knowledge_list_failed", err.Error())
	}

	for index, source := range request.Sources {
		normalized, warning, err := s.normalizeImportSource(request.SourceType, source)
		if err != nil {
			job.FailedSources = append(job.FailedSources, KnowledgeImportFailedSource{
				Name:   strings.TrimSpace(source.Name),
				URI:    strings.TrimSpace(source.URI),
				Reason: mapImportFailureCode(err.Error()),
			})
			return s.failJob(job, mapImportFailureCode(err.Error()), err.Error())
		}
		if existingID := existingDocumentIDByHash(documents, hashKnowledgeContent(normalized.Content)); existingID != "" {
			job.SkippedSources = append(job.SkippedSources, KnowledgeImportSkippedSource{
				Name:               normalized.Name,
				Reason:             KnowledgeImportSkipDuplicateContent,
				ExistingDocumentID: existingID,
			})
			continue
		}
		documentID := buildKnowledgeImportDocumentID(normalized.Name, now, index+1)
		metadata := map[string]string{
			"source_type":       string(request.SourceType),
			"created_from":      "knowledge_ingestion",
			"import_job_id":     job.ID,
			"source_uri":        sourceURIForImport(request.SourceType, normalized),
			"source_label":      job.SourceLabel,
			"source_size_bytes": strconv.FormatInt(normalized.SizeBytes, 10),
		}
		if warning != "" {
			metadata["source_warning"] = warning
		}
		document, err := s.knowledge.Upload(tenantID, KnowledgeUpload{
			ID:       documentID,
			Name:     normalized.Name,
			Content:  normalized.Content,
			SpaceID:  space.ID,
			Metadata: metadata,
		})
		if err != nil {
			job.FailedSources = append(job.FailedSources, KnowledgeImportFailedSource{
				Name:   normalized.Name,
				URI:    normalized.URI,
				Reason: "knowledge_upload_failed",
			})
			return s.failJob(job, "knowledge_upload_failed", err.Error())
		}
		job.ImportedDocumentIDs = append(job.ImportedDocumentIDs, document.ID)
		documents = append(documents, document)
	}

	job.Status = KnowledgeImportCompleted
	if len(job.ImportedDocumentIDs) > 0 && len(job.SkippedSources) > 0 {
		job.Status = KnowledgeImportPartial
	}
	job.UpdatedAt = s.now()
	saved, err := s.store.SaveKnowledgeImportJob(job)
	if err != nil {
		return KnowledgeImportJob{}, err
	}
	return saved, nil
}

func (s KnowledgeImportService) failJob(job KnowledgeImportJob, code, cause string) (KnowledgeImportJob, error) {
	job.Status = KnowledgeImportFailed
	job.FailureCode = strings.TrimSpace(code)
	job.FailureCause = strings.TrimSpace(cause)
	job.UpdatedAt = s.now()
	saved, err := s.store.SaveKnowledgeImportJob(job)
	if err != nil {
		return KnowledgeImportJob{}, err
	}
	return saved, fmt.Errorf("%s", cause)
}

func (s KnowledgeImportService) normalizeImportSource(sourceType KnowledgeImportSourceType, source KnowledgeImportSource) (KnowledgeImportSource, string, error) {
	content := strings.TrimSpace(source.Content)
	if content == "" {
		return KnowledgeImportSource{}, "", fmt.Errorf("knowledge import source content is required")
	}
	sizeBytes := source.SizeBytes
	if sizeBytes <= 0 {
		sizeBytes = int64(len([]byte(content)))
	}
	if sizeBytes > s.maxSourceBytes {
		return KnowledgeImportSource{}, "", fmt.Errorf("knowledge import source exceeds size limit")
	}
	name := strings.TrimSpace(source.Name)
	switch sourceType {
	case KnowledgeImportSourceLocalTextFile:
		lowerName := strings.ToLower(name)
		if !strings.HasSuffix(lowerName, ".txt") && !strings.HasSuffix(lowerName, ".md") {
			return KnowledgeImportSource{}, "", fmt.Errorf("unsupported source extension")
		}
	case KnowledgeImportSourceURLTextSnapshot:
		parsed, err := url.Parse(strings.TrimSpace(source.URI))
		if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || strings.TrimSpace(parsed.Host) == "" {
			return KnowledgeImportSource{}, "", fmt.Errorf("invalid source url")
		}
		if name == "" {
			name = nameFromImportURL(parsed)
		}
	default:
		return KnowledgeImportSource{}, "", fmt.Errorf("unsupported source type")
	}
	if name == "" {
		name = "imported.txt"
	}
	return KnowledgeImportSource{
		Name:      name,
		URI:       strings.TrimSpace(source.URI),
		Content:   content,
		SizeBytes: sizeBytes,
	}, detectImportWarning(content), nil
}

func isSupportedKnowledgeImportSourceType(sourceType KnowledgeImportSourceType) bool {
	switch sourceType {
	case KnowledgeImportSourceLocalTextFile, KnowledgeImportSourceURLTextSnapshot:
		return true
	default:
		return false
	}
}

func detectImportWarning(content string) string {
	lower := strings.ToLower(strings.TrimSpace(content))
	for _, needle := range []string{
		"ignore previous instructions",
		"ignore all previous instructions",
		"system prompt",
		"follow these instructions instead",
	} {
		if strings.Contains(lower, needle) {
			return KnowledgeImportWarningInstructionLikeText
		}
	}
	return ""
}

func existingDocumentIDByHash(documents []KnowledgeDocument, hash string) string {
	for _, document := range documents {
		if document.ContentHash == hash {
			return document.ID
		}
	}
	return ""
}

func buildKnowledgeImportDocumentID(name string, now time.Time, index int) string {
	base := sanitizeKnowledgeImportToken(strings.TrimSuffix(name, path.Ext(name)))
	if base == "" {
		base = "import"
	}
	return fmt.Sprintf("%s-%d-%d", base, now.UnixNano(), index)
}

func sanitizeKnowledgeImportToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			builder.WriteRune(r)
		case r == ' ' || r == '/' || r == '\\':
			builder.WriteRune('-')
		}
	}
	return strings.Trim(builder.String(), "-.")
}

func nameFromImportURL(parsed *url.URL) string {
	if parsed == nil {
		return "snapshot.txt"
	}
	trimmedPath := strings.Trim(strings.TrimSpace(parsed.Path), "/")
	if trimmedPath == "" {
		return parsed.Hostname() + ".txt"
	}
	token := sanitizeKnowledgeImportToken(strings.ReplaceAll(trimmedPath, "/", "-"))
	if token == "" {
		token = parsed.Hostname()
	}
	return token + ".txt"
}

func sourceURIForImport(sourceType KnowledgeImportSourceType, source KnowledgeImportSource) string {
	if sourceType == KnowledgeImportSourceURLTextSnapshot {
		return strings.TrimSpace(source.URI)
	}
	return strings.TrimSpace(source.Name)
}

func mapImportFailureCode(cause string) string {
	switch cause {
	case "unsupported source type":
		return "unsupported_source_type"
	case "unsupported source extension":
		return "unsupported_extension"
	case "invalid source url":
		return "invalid_url"
	case "knowledge import source content is required":
		return "empty_source_content"
	case "knowledge import source exceeds size limit":
		return "source_too_large"
	default:
		return "import_failed"
	}
}

func sortKnowledgeImportJobs(jobs []KnowledgeImportJob) {
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].CreatedAt.Equal(jobs[j].CreatedAt) {
			return jobs[i].ID > jobs[j].ID
		}
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
}

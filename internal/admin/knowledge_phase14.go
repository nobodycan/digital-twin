package admin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

type KnowledgeHealthStatus string

const (
	KnowledgeHealthHealthy        KnowledgeHealthStatus = "healthy"
	KnowledgeHealthNeedsAttention KnowledgeHealthStatus = "needs_attention"
	KnowledgeHealthEmpty          KnowledgeHealthStatus = "empty"
	KnowledgeHealthDisabled       KnowledgeHealthStatus = "disabled"
)

type KnowledgeHealthSummary struct {
	SpaceID               string                `json:"space_id"`
	SpaceName             string                `json:"space_name"`
	Status                KnowledgeHealthStatus `json:"status"`
	ActiveDocumentCount   int                   `json:"active_document_count"`
	DisabledDocumentCount int                   `json:"disabled_document_count"`
	FailedDocumentCount   int                   `json:"failed_document_count"`
	StaleDocumentCount    int                   `json:"stale_document_count"`
	ChunkCount            int                   `json:"chunk_count"`
	LastUpdatedAt         time.Time             `json:"last_updated_at"`
	AttentionReasons      []string              `json:"attention_reasons,omitempty"`
}

type KnowledgeDocumentDetail struct {
	Document     KnowledgeDocument          `json:"document"`
	QualityFlags []string                   `json:"quality_flags,omitempty"`
	Relations    KnowledgeDocumentRelations `json:"relations,omitempty"`
}

type KnowledgeDocumentRelations struct {
	SourceGap    *KnowledgeGap  `json:"source_gap,omitempty"`
	ResolvedGaps []KnowledgeGap `json:"resolved_gaps,omitempty"`
}

func (s KnowledgeService) HealthSummary(tenantID, spaceID string) (KnowledgeHealthSummary, error) {
	space, err := s.GetSpace(tenantID, spaceID)
	if err != nil {
		return KnowledgeHealthSummary{}, err
	}
	documents, err := s.ListBySpace(tenantID, space.ID)
	if err != nil {
		return KnowledgeHealthSummary{}, err
	}
	summary := KnowledgeHealthSummary{
		SpaceID:       space.ID,
		SpaceName:     space.Name,
		Status:        KnowledgeHealthHealthy,
		LastUpdatedAt: space.UpdatedAt,
	}
	if space.Status == KnowledgeSpaceDisabled || space.Status == KnowledgeSpaceArchived {
		summary.Status = KnowledgeHealthDisabled
		summary.AttentionReasons = append(summary.AttentionReasons, "space_not_active")
	}
	for _, document := range documents {
		summary.ChunkCount += document.ChunkCount
		if document.UpdatedAt.After(summary.LastUpdatedAt) {
			summary.LastUpdatedAt = document.UpdatedAt
		}
		switch document.Status {
		case KnowledgeReady:
			summary.ActiveDocumentCount++
		case KnowledgeDisabled:
			summary.DisabledDocumentCount++
		case KnowledgeFailed:
			summary.FailedDocumentCount++
			summary.AttentionReasons = appendIfMissing(summary.AttentionReasons, "failed_documents_present")
		default:
			summary.AttentionReasons = appendIfMissing(summary.AttentionReasons, "non_ready_documents_present")
		}
		vectorStatus := document.Metadata[KnowledgeMetadataVectorStatus]
		if vectorStatus == KnowledgeVectorFailed {
			summary.AttentionReasons = appendIfMissing(summary.AttentionReasons, "vector_index_failures_present")
		}
	}
	if summary.ActiveDocumentCount == 0 {
		summary.Status = KnowledgeHealthEmpty
		summary.AttentionReasons = appendIfMissing(summary.AttentionReasons, "no_active_documents")
	}
	if len(summary.AttentionReasons) > 0 && summary.Status == KnowledgeHealthHealthy {
		summary.Status = KnowledgeHealthNeedsAttention
	}
	return summary, nil
}

func (s KnowledgeService) DocumentDetail(tenantID, documentID string) (KnowledgeDocumentDetail, error) {
	return s.DocumentDetailWithRelations(tenantID, documentID, KnowledgeGapService{})
}

func (s KnowledgeService) DocumentDetailWithRelations(tenantID, documentID string, gaps KnowledgeGapService) (KnowledgeDocumentDetail, error) {
	document, err := s.Get(tenantID, documentID)
	if err != nil {
		return KnowledgeDocumentDetail{}, err
	}
	documents, err := s.ListBySpace(tenantID, document.SpaceID)
	if err != nil {
		return KnowledgeDocumentDetail{}, err
	}
	flags := make([]string, 0, 6)
	if document.Status == KnowledgeDisabled {
		flags = append(flags, "disabled")
	}
	if document.Status == KnowledgeFailed {
		flags = append(flags, "document_failed")
	}
	if effectiveKnowledgeReviewStatus(document) != KnowledgeReviewActive {
		flags = append(flags, "review_gated")
	}
	if document.ChunkCount == 0 || len(document.Chunks) == 0 {
		flags = append(flags, "no_chunks")
	}
	if strings.TrimSpace(document.Metadata["source_label"]) == "" {
		flags = append(flags, "missing_source_label")
	}
	if len(document.Chunks) > 0 && len(strings.TrimSpace(document.Chunks[0].Text)) > 0 && len(strings.TrimSpace(document.Chunks[0].Text)) < 32 {
		flags = append(flags, "short_content")
	}
	if document.Metadata[KnowledgeMetadataVectorStatus] == KnowledgeVectorMissing {
		flags = append(flags, "vector_missing")
	}
	if document.Metadata[KnowledgeMetadataVectorStatus] == KnowledgeVectorFailed {
		flags = append(flags, "index_failed")
	}
	if strings.TrimSpace(document.Metadata[KnowledgeMetadataLastErrorCode]) != "" {
		flags = append(flags, "last_error_present")
	}
	for _, candidate := range documents {
		if candidate.ID == document.ID {
			continue
		}
		if candidate.ContentHash != "" && candidate.ContentHash == document.ContentHash {
			flags = append(flags, "duplicate_content_hash")
			break
		}
	}
	relations, err := buildKnowledgeDocumentRelations(tenantID, document, gaps)
	if err != nil {
		return KnowledgeDocumentDetail{}, err
	}
	return KnowledgeDocumentDetail{
		Document:     document,
		QualityFlags: flags,
		Relations:    relations,
	}, nil
}

func buildKnowledgeDocumentRelations(tenantID string, document KnowledgeDocument, gaps KnowledgeGapService) (KnowledgeDocumentRelations, error) {
	if gaps.store == nil {
		return KnowledgeDocumentRelations{}, nil
	}
	records, err := gaps.List(tenantID, document.SpaceID)
	if err != nil {
		return KnowledgeDocumentRelations{}, err
	}
	relations := KnowledgeDocumentRelations{
		ResolvedGaps: make([]KnowledgeGap, 0, len(records)),
	}
	sourceGapID := strings.TrimSpace(document.Metadata["source_gap_id"])
	for _, gap := range records {
		if sourceGapID != "" && gap.ID == sourceGapID {
			gapCopy := gap
			relations.SourceGap = &gapCopy
		}
		if gap.ResolvedByDocumentID == document.ID {
			relations.ResolvedGaps = append(relations.ResolvedGaps, gap)
		}
	}
	sortKnowledgeGaps(relations.ResolvedGaps)
	return relations, nil
}

type KnowledgeGapStatus string

const (
	KnowledgeGapOpen          KnowledgeGapStatus = "open"
	KnowledgeGapInvestigating KnowledgeGapStatus = "investigating"
	KnowledgeGapIgnored       KnowledgeGapStatus = "ignored"
	KnowledgeGapResolved      KnowledgeGapStatus = "resolved"
)

type KnowledgeGap struct {
	ID                   string             `json:"id"`
	TenantID             string             `json:"tenant_id"`
	SpaceID              string             `json:"space_id"`
	Question             string             `json:"question"`
	NoSourceReason       string             `json:"no_source_reason"`
	Status               KnowledgeGapStatus `json:"status"`
	CreatedAt            time.Time          `json:"created_at"`
	UpdatedAt            time.Time          `json:"updated_at"`
	ResolvedByDocumentID string             `json:"resolved_by_document_id,omitempty"`
	ResolutionNote       string             `json:"resolution_note,omitempty"`
}

type KnowledgeGapInput struct {
	SpaceID        string `json:"space_id"`
	Question       string `json:"question"`
	NoSourceReason string `json:"no_source_reason"`
}

type KnowledgeGapStore interface {
	SaveKnowledgeGap(KnowledgeGap) (KnowledgeGap, error)
	ListKnowledgeGaps(tenantID, spaceID string) ([]KnowledgeGap, error)
	GetKnowledgeGap(tenantID, gapID string) (KnowledgeGap, error)
}

type KnowledgeGapService struct {
	store KnowledgeGapStore
	now   func() time.Time
}

func NewKnowledgeGapService(store KnowledgeGapStore) KnowledgeGapService {
	return KnowledgeGapService{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func (s KnowledgeGapService) Create(tenantID string, input KnowledgeGapInput) (KnowledgeGap, error) {
	spaceID := normalizeDocumentSpaceID(input.SpaceID)
	if err := validateKnowledgeID(spaceID); err != nil {
		return KnowledgeGap{}, err
	}
	question := strings.TrimSpace(input.Question)
	if question == "" {
		return KnowledgeGap{}, fmt.Errorf("knowledge gap question is required")
	}
	reason := strings.TrimSpace(input.NoSourceReason)
	if reason == "" {
		return KnowledgeGap{}, fmt.Errorf("knowledge gap no_source_reason is required")
	}
	existing, err := s.store.ListKnowledgeGaps(tenantID, spaceID)
	if err != nil {
		return KnowledgeGap{}, err
	}
	for _, gap := range existing {
		if gap.Status == KnowledgeGapOpen && gap.Question == question && gap.NoSourceReason == reason {
			return gap, nil
		}
	}
	now := s.now()
	id := fmt.Sprintf("gap-%d", now.UnixNano())
	return s.store.SaveKnowledgeGap(KnowledgeGap{
		ID:             id,
		TenantID:       tenantID,
		SpaceID:        spaceID,
		Question:       question,
		NoSourceReason: reason,
		Status:         KnowledgeGapOpen,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
}

func (s KnowledgeGapService) List(tenantID, spaceID string) ([]KnowledgeGap, error) {
	return s.store.ListKnowledgeGaps(tenantID, normalizeDocumentSpaceID(spaceID))
}

func (s KnowledgeGapService) Get(tenantID, gapID string) (KnowledgeGap, error) {
	if err := validateKnowledgeID(gapID); err != nil {
		return KnowledgeGap{}, err
	}
	return s.store.GetKnowledgeGap(tenantID, gapID)
}

func (s KnowledgeGapService) UpdateStatus(tenantID, gapID string, status KnowledgeGapStatus, resolvedByDocumentID, resolutionNote string) (KnowledgeGap, error) {
	if err := validateKnowledgeID(gapID); err != nil {
		return KnowledgeGap{}, err
	}
	switch status {
	case KnowledgeGapOpen, KnowledgeGapInvestigating, KnowledgeGapIgnored, KnowledgeGapResolved:
	default:
		return KnowledgeGap{}, fmt.Errorf("invalid knowledge gap status")
	}
	if resolvedByDocumentID != "" {
		if err := validateKnowledgeID(resolvedByDocumentID); err != nil {
			return KnowledgeGap{}, err
		}
	}
	gap, err := s.store.GetKnowledgeGap(tenantID, gapID)
	if err != nil {
		return KnowledgeGap{}, err
	}
	gap.Status = status
	gap.UpdatedAt = s.now()
	if status == KnowledgeGapResolved {
		gap.ResolvedByDocumentID = resolvedByDocumentID
		gap.ResolutionNote = strings.TrimSpace(resolutionNote)
	} else {
		gap.ResolvedByDocumentID = ""
		gap.ResolutionNote = ""
	}
	return s.store.SaveKnowledgeGap(gap)
}

type InMemoryKnowledgeGapStore struct {
	mu   sync.Mutex
	gaps map[string]map[string]KnowledgeGap
}

func NewInMemoryKnowledgeGapStore() *InMemoryKnowledgeGapStore {
	return &InMemoryKnowledgeGapStore{
		gaps: make(map[string]map[string]KnowledgeGap),
	}
}

func (s *InMemoryKnowledgeGapStore) SaveKnowledgeGap(gap KnowledgeGap) (KnowledgeGap, error) {
	if err := validateKnowledgeID(gap.ID); err != nil {
		return KnowledgeGap{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.gaps[gap.TenantID]; !ok {
		s.gaps[gap.TenantID] = make(map[string]KnowledgeGap)
	}
	gap.SpaceID = normalizeDocumentSpaceID(gap.SpaceID)
	s.gaps[gap.TenantID][gap.ID] = gap
	return gap, nil
}

func (s *InMemoryKnowledgeGapStore) ListKnowledgeGaps(tenantID, spaceID string) ([]KnowledgeGap, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.gaps[tenantID]
	out := make([]KnowledgeGap, 0, len(items))
	for _, gap := range items {
		if normalizeDocumentSpaceID(spaceID) == DefaultKnowledgeSpaceID && gap.SpaceID == DefaultKnowledgeSpaceID || gap.SpaceID == normalizeDocumentSpaceID(spaceID) {
			out = append(out, gap)
		}
	}
	sortKnowledgeGaps(out)
	return out, nil
}

func (s *InMemoryKnowledgeGapStore) GetKnowledgeGap(tenantID, gapID string) (KnowledgeGap, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	gap, ok := s.gaps[tenantID][gapID]
	if !ok {
		return KnowledgeGap{}, ErrKnowledgeDocumentNotFound
	}
	return gap, nil
}

type FileKnowledgeGapStore struct {
	dir string
}

func NewFileKnowledgeGapStore(dir string) *FileKnowledgeGapStore {
	return &FileKnowledgeGapStore{dir: dir}
}

func (s *FileKnowledgeGapStore) SaveKnowledgeGap(gap KnowledgeGap) (KnowledgeGap, error) {
	if err := validateKnowledgeID(gap.ID); err != nil {
		return KnowledgeGap{}, err
	}
	gaps, err := s.load()
	if err != nil {
		return KnowledgeGap{}, err
	}
	replaced := false
	for i, existing := range gaps {
		if existing.TenantID == gap.TenantID && existing.ID == gap.ID {
			gaps[i] = gap
			replaced = true
			break
		}
	}
	if !replaced {
		gaps = append(gaps, gap)
	}
	sortKnowledgeGaps(gaps)
	if err := s.save(gaps); err != nil {
		return KnowledgeGap{}, err
	}
	return gap, nil
}

func (s *FileKnowledgeGapStore) ListKnowledgeGaps(tenantID, spaceID string) ([]KnowledgeGap, error) {
	gaps, err := s.load()
	if err != nil {
		return nil, err
	}
	out := make([]KnowledgeGap, 0, len(gaps))
	normalizedSpaceID := normalizeDocumentSpaceID(spaceID)
	for _, gap := range gaps {
		if gap.TenantID == tenantID && gap.SpaceID == normalizedSpaceID {
			out = append(out, gap)
		}
	}
	sortKnowledgeGaps(out)
	return out, nil
}

func (s *FileKnowledgeGapStore) GetKnowledgeGap(tenantID, gapID string) (KnowledgeGap, error) {
	if err := validateKnowledgeID(gapID); err != nil {
		return KnowledgeGap{}, err
	}
	gaps, err := s.load()
	if err != nil {
		return KnowledgeGap{}, err
	}
	for _, gap := range gaps {
		if gap.TenantID == tenantID && gap.ID == gapID {
			return gap, nil
		}
	}
	return KnowledgeGap{}, ErrKnowledgeDocumentNotFound
}

func (s *FileKnowledgeGapStore) load() ([]KnowledgeGap, error) {
	data, err := os.ReadFile(s.path())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var gaps []KnowledgeGap
	if err := json.Unmarshal(data, &gaps); err != nil {
		return nil, err
	}
	sortKnowledgeGaps(gaps)
	return gaps, nil
}

func (s *FileKnowledgeGapStore) save(gaps []KnowledgeGap) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(gaps, "", "  ")
	if err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp(s.dir, "knowledge_gaps.json.*.tmp")
	if err != nil {
		return err
	}
	tmp := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, s.path()); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (s *FileKnowledgeGapStore) path() string {
	return filepath.Join(s.dir, "knowledge_gaps.json")
}

func sortKnowledgeGaps(gaps []KnowledgeGap) {
	sort.Slice(gaps, func(i, j int) bool {
		if gaps[i].CreatedAt.Equal(gaps[j].CreatedAt) {
			return gaps[i].ID < gaps[j].ID
		}
		return gaps[i].CreatedAt.Before(gaps[j].CreatedAt)
	})
}

func appendIfMissing(values []string, value string) []string {
	if slices.Contains(values, value) {
		return values
	}
	return append(values, value)
}

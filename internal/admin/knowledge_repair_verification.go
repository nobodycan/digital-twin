package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type RepairVerificationResult string

const (
	RepairVerificationPassed RepairVerificationResult = "passed"
	RepairVerificationFailed RepairVerificationResult = "failed"
)

type RepairVerificationFailureReason string

const (
	RepairVerificationNoActiveSource RepairVerificationFailureReason = "no_active_source"
	RepairVerificationReviewGated    RepairVerificationFailureReason = "review_gated"
	RepairVerificationUnsupported    RepairVerificationFailureReason = "unsupported"
	RepairVerificationRetrievalError RepairVerificationFailureReason = "retrieval_error"
)

type RepairVerificationSource struct {
	DocumentID   string `json:"document_id"`
	Title        string `json:"title,omitempty"`
	ReviewStatus string `json:"review_status,omitempty"`
	SourceType   string `json:"source_type,omitempty"`
}

type RepairVerificationAttempt struct {
	ID                           string                          `json:"id"`
	TenantID                     string                          `json:"tenant_id"`
	GapID                        string                          `json:"gap_id"`
	SpaceID                      string                          `json:"space_id"`
	StartedAt                    time.Time                       `json:"started_at"`
	CompletedAt                  time.Time                       `json:"completed_at"`
	BeforeState                  string                          `json:"before_state"`
	AfterState                   string                          `json:"after_state"`
	KnowledgeSnapshotFingerprint string                          `json:"knowledge_snapshot_fingerprint"`
	EvidenceFingerprint          string                          `json:"evidence_fingerprint"`
	SourceCount                  int                             `json:"source_count"`
	TopSources                   []RepairVerificationSource      `json:"top_sources,omitempty"`
	Result                       RepairVerificationResult        `json:"result"`
	FailureReason                RepairVerificationFailureReason `json:"failure_reason,omitempty"`
}

type RepairVerificationState string

const (
	RepairVerificationUnverified RepairVerificationState = "unverified"
	RepairVerificationVerified   RepairVerificationState = "verified"
	RepairVerificationStale      RepairVerificationState = "stale"
)

type RepairVerificationStore interface {
	AppendRepairVerification(RepairVerificationAttempt) (RepairVerificationAttempt, error)
	ListRepairVerifications(tenantID, gapID string) ([]RepairVerificationAttempt, error)
}

type RepairVerificationDiagnosticRequest struct {
	Query   string
	Limit   int
	SpaceID string
}

type RepairVerificationDiagnosticResult struct {
	DocumentID   string
	DocumentName string
	SourceType   string
	ReviewStatus string
	ChunkID      string
	Rank         int
}

type RepairVerificationDiagnosticResponse struct {
	Results        []RepairVerificationDiagnosticResult
	NoSourceReason string
	ReviewGated    int
}

type RepairVerificationDiagnosticRunner func(context.Context, string, RepairVerificationDiagnosticRequest) (RepairVerificationDiagnosticResponse, error)

type RepairVerificationService struct {
	store     RepairVerificationStore
	gaps      KnowledgeGapService
	knowledge KnowledgeService
	diagnose  RepairVerificationDiagnosticRunner
}

type RepairVerificationProjection struct {
	State          RepairVerificationState
	LastVerifiedAt *time.Time
	LastResult     RepairVerificationResult
	LastFailure    RepairVerificationFailureReason
}

func NewRepairVerificationService(store RepairVerificationStore, gaps KnowledgeGapService, knowledge KnowledgeService, diagnose RepairVerificationDiagnosticRunner) RepairVerificationService {
	return RepairVerificationService{store: store, gaps: gaps, knowledge: knowledge, diagnose: diagnose}
}

func (s RepairVerificationService) Verify(ctx context.Context, tenantID, gapID string) (RepairVerificationAttempt, error) {
	if s.store == nil || s.diagnose == nil {
		return RepairVerificationAttempt{}, errors.New("repair verification service unavailable")
	}
	gap, err := s.gaps.Get(tenantID, gapID)
	if err != nil {
		return RepairVerificationAttempt{}, err
	}
	documents, err := s.knowledge.ListBySpace(tenantID, gap.SpaceID)
	if err != nil {
		return RepairVerificationAttempt{}, err
	}
	snapshot, err := repairVerificationSnapshotFingerprint(documents, gap.SpaceID)
	if err != nil {
		return RepairVerificationAttempt{}, err
	}
	started := time.Now().UTC()
	attempt := RepairVerificationAttempt{
		ID: fmt.Sprintf("verification-%d", started.UnixNano()), TenantID: tenantID, GapID: gap.ID, SpaceID: gap.SpaceID,
		StartedAt: started, CompletedAt: started, BeforeState: s.beforeState(tenantID, gap.ID), KnowledgeSnapshotFingerprint: snapshot,
	}
	response, err := s.diagnose(ctx, tenantID, RepairVerificationDiagnosticRequest{Query: gap.Question, Limit: 3, SpaceID: gap.SpaceID})
	if err != nil {
		attempt.AfterState = "unknown"
		attempt.Result = RepairVerificationFailed
		attempt.FailureReason = RepairVerificationRetrievalError
		attempt.EvidenceFingerprint, err = hashRepairVerificationPayload("repair_evidence_v1", []any{})
		if err != nil {
			return RepairVerificationAttempt{}, err
		}
		return s.store.AppendRepairVerification(attempt)
	}
	attempt.EvidenceFingerprint, err = repairVerificationEvidenceFingerprint(response.Results, documents)
	if err != nil {
		return RepairVerificationAttempt{}, err
	}
	attempt.SourceCount = len(response.Results)
	attempt.TopSources = safeRepairVerificationSources(response.Results)
	if len(response.Results) > 0 {
		attempt.Result = RepairVerificationPassed
		attempt.AfterState = "grounded"
	} else {
		attempt.Result = RepairVerificationFailed
		if response.ReviewGated > 0 || response.NoSourceReason == "no_review_active_documents" {
			attempt.FailureReason = RepairVerificationReviewGated
			attempt.AfterState = "review_gated"
		} else if response.NoSourceReason == "no_ready_documents" {
			attempt.FailureReason = RepairVerificationNoActiveSource
			attempt.AfterState = "no_active_source"
		} else {
			attempt.FailureReason = RepairVerificationUnsupported
			attempt.AfterState = "unsupported"
		}
	}
	return s.store.AppendRepairVerification(attempt)
}

func (s RepairVerificationService) List(tenantID, gapID string, limit int) ([]RepairVerificationAttempt, error) {
	if s.store == nil {
		return nil, errors.New("repair verification service unavailable")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	items, err := s.store.ListRepairVerifications(tenantID, gapID)
	if err != nil {
		return nil, err
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (s RepairVerificationService) Project(tenantID string, gap KnowledgeGap, documents []KnowledgeDocument) (RepairVerificationProjection, error) {
	items, err := s.List(tenantID, gap.ID, 100)
	if err != nil {
		return RepairVerificationProjection{}, err
	}
	projection := RepairVerificationProjection{State: RepairVerificationUnverified}
	if len(items) == 0 {
		return projection, nil
	}
	latest := items[0]
	projection.LastResult = latest.Result
	projection.LastFailure = latest.FailureReason
	for _, item := range items {
		if item.Result == RepairVerificationPassed {
			verifiedAt := item.CompletedAt
			projection.LastVerifiedAt = &verifiedAt
			current, fingerprintErr := repairVerificationSnapshotFingerprint(documents, gap.SpaceID)
			if fingerprintErr == nil && current == item.KnowledgeSnapshotFingerprint {
				projection.State = RepairVerificationVerified
			} else {
				projection.State = RepairVerificationStale
			}
			break
		}
	}
	return projection, nil
}

func (s RepairVerificationService) beforeState(tenantID, gapID string) string {
	items, err := s.store.ListRepairVerifications(tenantID, gapID)
	if err == nil && len(items) > 0 && strings.TrimSpace(items[0].AfterState) != "" {
		return items[0].AfterState
	}
	return "unknown"
}

type repairVerificationEvidenceEntry struct {
	DocumentID   string `json:"document_id"`
	ChunkID      string `json:"chunk_id"`
	ContentHash  string `json:"content_hash"`
	Rank         int    `json:"rank"`
	ReviewStatus string `json:"review_status"`
}

func repairVerificationEvidenceFingerprint(results []RepairVerificationDiagnosticResult, documents []KnowledgeDocument) (string, error) {
	hashes := make(map[string]string, len(documents))
	for _, document := range documents {
		hashes[document.ID] = document.ContentHash
	}
	entries := make([]repairVerificationEvidenceEntry, 0, len(results))
	for _, result := range results {
		entries = append(entries, repairVerificationEvidenceEntry{DocumentID: result.DocumentID, ChunkID: result.ChunkID, ContentHash: hashes[result.DocumentID], Rank: result.Rank, ReviewStatus: result.ReviewStatus})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Rank != entries[j].Rank {
			return entries[i].Rank < entries[j].Rank
		}
		if entries[i].DocumentID != entries[j].DocumentID {
			return entries[i].DocumentID < entries[j].DocumentID
		}
		return entries[i].ChunkID < entries[j].ChunkID
	})
	return hashRepairVerificationPayload("repair_evidence_v1", entries)
}

func safeRepairVerificationSources(results []RepairVerificationDiagnosticResult) []RepairVerificationSource {
	if len(results) > 3 {
		results = results[:3]
	}
	out := make([]RepairVerificationSource, 0, len(results))
	for _, result := range results {
		out = append(out, RepairVerificationSource{DocumentID: result.DocumentID, Title: result.DocumentName, ReviewStatus: result.ReviewStatus, SourceType: result.SourceType})
	}
	return out
}

type InMemoryRepairVerificationStore struct {
	mu       sync.Mutex
	attempts []RepairVerificationAttempt
}

func NewInMemoryRepairVerificationStore() *InMemoryRepairVerificationStore {
	return &InMemoryRepairVerificationStore{}
}

func (s *InMemoryRepairVerificationStore) AppendRepairVerification(attempt RepairVerificationAttempt) (RepairVerificationAttempt, error) {
	if err := validateRepairVerificationAttempt(attempt); err != nil {
		return RepairVerificationAttempt{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts = append(s.attempts, cloneRepairVerificationAttempt(attempt))
	return attempt, nil
}

func (s *InMemoryRepairVerificationStore) ListRepairVerifications(tenantID, gapID string) ([]RepairVerificationAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return filterRepairVerificationAttempts(s.attempts, tenantID, gapID), nil
}

type FileRepairVerificationStore struct {
	dir string
	mu  sync.Mutex
}

func NewFileRepairVerificationStore(dir string) *FileRepairVerificationStore {
	return &FileRepairVerificationStore{dir: dir}
}

func (s *FileRepairVerificationStore) AppendRepairVerification(attempt RepairVerificationAttempt) (RepairVerificationAttempt, error) {
	if err := validateRepairVerificationAttempt(attempt); err != nil {
		return RepairVerificationAttempt{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	attempts, err := s.load()
	if err != nil {
		return RepairVerificationAttempt{}, err
	}
	attempts = append(attempts, attempt)
	if err := s.save(attempts); err != nil {
		return RepairVerificationAttempt{}, err
	}
	return attempt, nil
}

func (s *FileRepairVerificationStore) ListRepairVerifications(tenantID, gapID string) ([]RepairVerificationAttempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	attempts, err := s.load()
	if err != nil {
		return nil, err
	}
	return filterRepairVerificationAttempts(attempts, tenantID, gapID), nil
}

func (s *FileRepairVerificationStore) load() ([]RepairVerificationAttempt, error) {
	data, err := os.ReadFile(s.path())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var attempts []RepairVerificationAttempt
	if err := json.Unmarshal(data, &attempts); err != nil {
		return nil, err
	}
	return attempts, nil
}

func (s *FileRepairVerificationStore) save(attempts []RepairVerificationAttempt) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(attempts, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, "repair_verifications.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path())
}

func (s *FileRepairVerificationStore) path() string {
	return filepath.Join(s.dir, "repair_verifications.json")
}

func validateRepairVerificationAttempt(attempt RepairVerificationAttempt) error {
	if err := validateKnowledgeID(attempt.ID); err != nil {
		return fmt.Errorf("verification id: %w", err)
	}
	if err := validateKnowledgeID(attempt.TenantID); err != nil {
		return fmt.Errorf("verification tenant: %w", err)
	}
	if err := validateKnowledgeID(attempt.GapID); err != nil {
		return fmt.Errorf("verification gap: %w", err)
	}
	if attempt.CompletedAt.IsZero() {
		return errors.New("verification completed_at is required")
	}
	return nil
}

func filterRepairVerificationAttempts(attempts []RepairVerificationAttempt, tenantID, gapID string) []RepairVerificationAttempt {
	out := make([]RepairVerificationAttempt, 0)
	for _, attempt := range attempts {
		if attempt.TenantID == tenantID && (gapID == "" || attempt.GapID == gapID) {
			out = append(out, cloneRepairVerificationAttempt(attempt))
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CompletedAt.After(out[j].CompletedAt) })
	return out
}

func cloneRepairVerificationAttempt(attempt RepairVerificationAttempt) RepairVerificationAttempt {
	attempt.TopSources = append([]RepairVerificationSource(nil), attempt.TopSources...)
	return attempt
}

type repairVerificationSnapshotEntry struct {
	DocumentID   string `json:"document_id"`
	ContentHash  string `json:"content_hash"`
	UpdatedAt    string `json:"updated_at"`
	Status       string `json:"status"`
	ReviewStatus string `json:"review_status"`
	LexicalReady string `json:"lexical_ready"`
	VectorStatus string `json:"vector_status"`
}

func repairVerificationSnapshotFingerprint(documents []KnowledgeDocument, spaceID string) (string, error) {
	entries := make([]repairVerificationSnapshotEntry, 0, len(documents))
	for _, document := range documents {
		if normalizeDocumentSpaceID(document.SpaceID) != normalizeDocumentSpaceID(spaceID) || document.Status != KnowledgeReady || effectiveKnowledgeReviewStatus(document) != KnowledgeReviewActive {
			continue
		}
		entries = append(entries, repairVerificationSnapshotEntry{
			DocumentID: document.ID, ContentHash: document.ContentHash,
			UpdatedAt: document.UpdatedAt.UTC().Format(time.RFC3339Nano),
			Status:    string(document.Status), ReviewStatus: string(effectiveKnowledgeReviewStatus(document)),
			LexicalReady: document.Metadata[KnowledgeMetadataLexicalReady], VectorStatus: document.Metadata[KnowledgeMetadataVectorStatus],
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].DocumentID < entries[j].DocumentID })
	return hashRepairVerificationPayload("repair_snapshot_v1", entries)
}

func hashRepairVerificationPayload(version string, value any) (string, error) {
	payload, err := json.Marshal(struct {
		Version string `json:"version"`
		Value   any    `json:"value"`
	}{version, value})
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(hash[:]), nil
}

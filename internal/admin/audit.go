package admin

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type AuditStatus string

const (
	AuditStatusCompleted AuditStatus = "completed"
	AuditStatusCancelled AuditStatus = "cancelled"
	AuditStatusFailed    AuditStatus = "failed"
)

type AuditRecord struct {
	ID                      string         `json:"id"`
	TenantID                string         `json:"tenant_id"`
	ConversationID          string         `json:"conversation_id"`
	UserID                  string         `json:"user_id"`
	Status                  AuditStatus    `json:"status"`
	AgentName               string         `json:"agent_name"`
	LatencyMS               int64          `json:"latency_ms"`
	EventSummary            []string       `json:"event_summary"`
	QuestionSummary         string         `json:"question_summary,omitempty"`
	KnowledgeSpaceID        string         `json:"knowledge_space_id,omitempty"`
	KnowledgeNoSourceReason string         `json:"knowledge_no_source_reason,omitempty"`
	KnowledgeAnswerState    string         `json:"knowledge_answer_state,omitempty"`
	KnowledgeSourceCount    int            `json:"knowledge_source_count,omitempty"`
	KnowledgeEvidence       map[string]any `json:"knowledge_evidence,omitempty"`
	CreatedAt               time.Time      `json:"created_at"`
}

type AnswerAuditTimelineFilter struct {
	State          string
	WeakOnly       bool
	DocumentID     string
	ConversationID string
	Limit          int
}

type AnswerAuditTimelineItem struct {
	AuditID          string                         `json:"audit_id"`
	ConversationID   string                         `json:"conversation_id"`
	UserID           string                         `json:"user_id"`
	CreatedAt        time.Time                      `json:"created_at"`
	Status           AuditStatus                    `json:"status"`
	AgentName        string                         `json:"agent_name"`
	LatencyMS        int64                          `json:"latency_ms"`
	AnswerState      string                         `json:"answer_state"`
	SourceCount      int                            `json:"source_count"`
	QuestionSummary  string                         `json:"question_summary,omitempty"`
	KnowledgeSpaceID string                         `json:"-"`
	Summary          string                         `json:"summary"`
	TopSources       []AnswerAuditTimelineSource    `json:"top_sources,omitempty"`
	Diagnostics      AnswerAuditTimelineDiagnostics `json:"diagnostics"`
	Gap              *AnswerAuditTimelineGap        `json:"gap,omitempty"`
}

type AnswerAuditTimelineSource struct {
	DocumentID   string `json:"document_id,omitempty"`
	Title        string `json:"title,omitempty"`
	ReviewStatus string `json:"review_status,omitempty"`
	SourceType   string `json:"source_type,omitempty"`
	Snippet      string `json:"snippet,omitempty"`
}

type AnswerAuditTimelineDiagnostics struct {
	NoSourceReason   string `json:"no_source_reason,omitempty"`
	ReviewGatedCount int    `json:"review_gated_count,omitempty"`
}

type AnswerAuditTimelineGap struct {
	GapID  string `json:"gap_id,omitempty"`
	Status string `json:"status,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type AuditStore interface {
	SaveAudit(AuditRecord) (AuditRecord, error)
	ListAudit(tenantID string) ([]AuditRecord, error)
}

type AuditService struct {
	store AuditStore
	now   func() time.Time
}

func NewAuditService(store AuditStore) AuditService {
	return AuditService{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func (s AuditService) Record(tenantID string, record AuditRecord) (AuditRecord, error) {
	record.TenantID = tenantID
	if record.ID == "" {
		record.ID = fmt.Sprintf("audit-%s", record.ConversationID)
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = s.now()
	}
	return s.store.SaveAudit(record)
}

func (s AuditService) Recent(tenantID string) ([]AuditRecord, error) {
	records, err := s.store.ListAudit(tenantID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(records, func(i, j int) bool {
		if !records[i].CreatedAt.Equal(records[j].CreatedAt) {
			return records[i].CreatedAt.After(records[j].CreatedAt)
		}
		return records[i].ID > records[j].ID
	})
	return records, nil
}

func (s AuditService) Timeline(tenantID string, filter AnswerAuditTimelineFilter) ([]AnswerAuditTimelineItem, error) {
	records, err := s.Recent(tenantID)
	if err != nil {
		return nil, err
	}
	items := make([]AnswerAuditTimelineItem, 0, len(records))
	for _, record := range records {
		item := timelineItemFromRecord(record)
		if !matchesTimelineFilter(item, filter) {
			continue
		}
		items = append(items, item)
		if filter.Limit > 0 && len(items) >= filter.Limit {
			break
		}
	}
	return items, nil
}

func timelineItemFromRecord(record AuditRecord) AnswerAuditTimelineItem {
	evidence := record.KnowledgeEvidence
	answerState := strings.TrimSpace(record.KnowledgeAnswerState)
	if answerState == "" {
		answerState = strings.TrimSpace(stringValue(evidence["answer_state"]))
	}
	if answerState == "" {
		answerState = "unknown"
	}
	summary := strings.TrimSpace(stringValue(evidence["summary"]))
	if summary == "" {
		summary = "No supporting evidence recorded"
	}
	citations := citationsFromEvidence(evidence["citations"])
	sourceCount := record.KnowledgeSourceCount
	if sourceCount == 0 {
		sourceCount = len(citations)
	}
	item := AnswerAuditTimelineItem{
		AuditID:          record.ID,
		ConversationID:   record.ConversationID,
		UserID:           record.UserID,
		CreatedAt:        record.CreatedAt,
		Status:           record.Status,
		AgentName:        record.AgentName,
		LatencyMS:        record.LatencyMS,
		AnswerState:      answerState,
		SourceCount:      sourceCount,
		QuestionSummary:  strings.TrimSpace(record.QuestionSummary),
		KnowledgeSpaceID: strings.TrimSpace(record.KnowledgeSpaceID),
		Summary:          summary,
		TopSources:       citations,
		Diagnostics:      diagnosticsFromEvidence(record, evidence["diagnostics"]),
		Gap:              gapFromEvidence(evidence["gaps"]),
	}
	return item
}

func matchesTimelineFilter(item AnswerAuditTimelineItem, filter AnswerAuditTimelineFilter) bool {
	if filter.ConversationID != "" && item.ConversationID != strings.TrimSpace(filter.ConversationID) {
		return false
	}
	if filter.State != "" && item.AnswerState != strings.TrimSpace(filter.State) {
		return false
	}
	if filter.WeakOnly && !isWeakAnswerState(item.AnswerState) {
		return false
	}
	if filter.DocumentID != "" {
		documentID := strings.TrimSpace(filter.DocumentID)
		matched := false
		for _, source := range item.TopSources {
			if source.DocumentID == documentID {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func isWeakAnswerState(state string) bool {
	switch strings.TrimSpace(state) {
	case "unsupported", "partially_supported", "review_gated", "provider_fallback", "guard_rejected":
		return true
	default:
		return false
	}
}

func citationsFromEvidence(value any) []AnswerAuditTimelineSource {
	items, ok := value.([]map[string]any)
	if !ok {
		return nil
	}
	sources := make([]AnswerAuditTimelineSource, 0, len(items))
	for _, item := range items {
		sources = append(sources, AnswerAuditTimelineSource{
			DocumentID:   stringValue(item["document_id"]),
			Title:        stringValue(item["title"]),
			ReviewStatus: stringValue(item["review_status"]),
			SourceType:   stringValue(item["source_type"]),
			Snippet:      stringValue(item["snippet"]),
		})
	}
	return sources
}

func diagnosticsFromEvidence(record AuditRecord, value any) AnswerAuditTimelineDiagnostics {
	diagnostics := AnswerAuditTimelineDiagnostics{
		NoSourceReason: strings.TrimSpace(record.KnowledgeNoSourceReason),
	}
	mapped, ok := value.(map[string]any)
	if !ok {
		return diagnostics
	}
	if diagnostics.NoSourceReason == "" {
		diagnostics.NoSourceReason = stringValue(mapped["no_source_reason"])
	}
	diagnostics.ReviewGatedCount = intValue(mapped["review_gated_count"])
	return diagnostics
}

func gapFromEvidence(value any) *AnswerAuditTimelineGap {
	items, ok := value.([]map[string]any)
	if !ok || len(items) == 0 {
		return nil
	}
	first := items[0]
	gapID := stringValue(first["gap_id"])
	status := stringValue(first["status"])
	reason := stringValue(first["reason"])
	if gapID == "" && status == "" && reason == "" {
		return nil
	}
	return &AnswerAuditTimelineGap{GapID: gapID, Status: status, Reason: reason}
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

type InMemoryAuditStore struct {
	mu      sync.Mutex
	records map[string][]AuditRecord
}

func NewInMemoryAuditStore() *InMemoryAuditStore {
	return &InMemoryAuditStore{records: make(map[string][]AuditRecord)}
}

func (s *InMemoryAuditStore) SaveAudit(record AuditRecord) (AuditRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[record.TenantID] = append(s.records[record.TenantID], record)
	return record, nil
}

func (s *InMemoryAuditStore) ListAudit(tenantID string) ([]AuditRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := s.records[tenantID]
	out := make([]AuditRecord, len(records))
	copy(out, records)
	return out, nil
}

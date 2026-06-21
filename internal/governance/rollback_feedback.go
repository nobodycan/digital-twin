package governance

import (
	"sort"
	"sync"
	"time"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/pkg/types"
)

type RollbackRecord struct {
	ID              string        `json:"id"`
	TenantID        string        `json:"tenant_id"`
	CandidateType   CandidateType `json:"candidate_type"`
	PreviousVersion string        `json:"previous_version"`
	TargetVersion   string        `json:"target_version"`
	ActorID         string        `json:"actor_id"`
	CreatedAt       time.Time     `json:"created_at"`
}

type RollbackService struct {
	Decisions DecisionStore
}

func (s RollbackService) Record(record RollbackRecord) (RollbackRecord, error) {
	if err := validateRollbackRecord(record); err != nil {
		return RollbackRecord{}, err
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if s.Decisions != nil {
		if err := s.Decisions.SaveDecision(DecisionRecord{
			ID:        "rollback-" + record.ID,
			TenantID:  record.TenantID,
			Type:      DecisionRollback,
			ActorID:   record.ActorID,
			CreatedAt: record.CreatedAt,
			Evidence: types.Metadata{
				"rollback_id":      record.ID,
				"candidate_type":   record.CandidateType,
				"previous_version": record.PreviousVersion,
				"target_version":   record.TargetVersion,
			},
		}); err != nil {
			return RollbackRecord{}, err
		}
	}
	return record, nil
}

func validateRollbackRecord(record RollbackRecord) error {
	for field, value := range map[string]string{
		"rollback_id":      record.ID,
		"tenant_id":        record.TenantID,
		"previous_version": record.PreviousVersion,
		"target_version":   record.TargetVersion,
		"actor_id":         record.ActorID,
	} {
		if err := validateSafeID(field, value); err != nil {
			return err
		}
	}
	if !validCandidateType(record.CandidateType) {
		return validateSafeID("candidate_type", "")
	}
	return nil
}

type FeedbackCategory string

const (
	FeedbackPersona   FeedbackCategory = "persona"
	FeedbackKnowledge FeedbackCategory = "knowledge"
	FeedbackTool      FeedbackCategory = "tool"
	FeedbackSafety    FeedbackCategory = "safety"
)

type FeedbackSeverity string

const (
	FeedbackSeverityLow    FeedbackSeverity = "low"
	FeedbackSeverityMedium FeedbackSeverity = "medium"
	FeedbackSeverityHigh   FeedbackSeverity = "high"
)

type FeedbackStatus string

const (
	FeedbackNew                FeedbackStatus = "new"
	FeedbackTriaged            FeedbackStatus = "triaged"
	FeedbackEvalAdded          FeedbackStatus = "eval-added"
	FeedbackKnowledgeFixNeeded FeedbackStatus = "knowledge-fix-needed"
	FeedbackPersonaFixNeeded   FeedbackStatus = "persona-fix-needed"
	FeedbackDismissed          FeedbackStatus = "dismissed"
	FeedbackResolved           FeedbackStatus = "resolved"
)

type FeedbackRecord struct {
	ID               string           `json:"id"`
	TenantID         string           `json:"tenant_id"`
	ConversationID   string           `json:"conversation_id"`
	MessageID        string           `json:"message_id"`
	Category         FeedbackCategory `json:"category"`
	Severity         FeedbackSeverity `json:"severity"`
	Note             string           `json:"note"`
	Status           FeedbackStatus   `json:"status"`
	LinkedEvalCaseID string           `json:"linked_eval_case_id,omitempty"`
	CreatedBy        string           `json:"created_by"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

type FeedbackStore interface {
	SaveFeedback(FeedbackRecord) (FeedbackRecord, error)
	GetFeedback(tenantID, feedbackID string) (FeedbackRecord, error)
	ListFeedback(tenantID string) ([]FeedbackRecord, error)
}

type FeedbackService struct {
	Store FeedbackStore
}

func (s FeedbackService) Create(record FeedbackRecord) (FeedbackRecord, error) {
	if record.Status == "" {
		record.Status = FeedbackNew
	}
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	}
	if err := validateFeedbackRecord(record); err != nil {
		return FeedbackRecord{}, err
	}
	return s.Store.SaveFeedback(record)
}

func (s FeedbackService) UpdateStatus(tenantID, feedbackID string, status FeedbackStatus, linkedEvalCaseID string) (FeedbackRecord, error) {
	record, err := s.Store.GetFeedback(tenantID, feedbackID)
	if err != nil {
		return FeedbackRecord{}, err
	}
	record.Status = status
	record.LinkedEvalCaseID = linkedEvalCaseID
	record.UpdatedAt = time.Now().UTC()
	if err := validateFeedbackRecord(record); err != nil {
		return FeedbackRecord{}, err
	}
	return s.Store.SaveFeedback(record)
}

type InMemoryFeedbackStore struct {
	mu      sync.Mutex
	records map[string]FeedbackRecord
}

func NewInMemoryFeedbackStore() *InMemoryFeedbackStore {
	return &InMemoryFeedbackStore{records: make(map[string]FeedbackRecord)}
}

func (s *InMemoryFeedbackStore) SaveFeedback(record FeedbackRecord) (FeedbackRecord, error) {
	if err := validateFeedbackRecord(record); err != nil {
		return FeedbackRecord{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[decisionKey(record.TenantID, record.ID)] = record
	return record, nil
}

func (s *InMemoryFeedbackStore) GetFeedback(tenantID, feedbackID string) (FeedbackRecord, error) {
	if err := validateSafeID("tenant_id", tenantID); err != nil {
		return FeedbackRecord{}, err
	}
	if err := validateSafeID("feedback_id", feedbackID); err != nil {
		return FeedbackRecord{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	feedback, ok := s.records[decisionKey(tenantID, feedbackID)]
	if !ok {
		return FeedbackRecord{}, core.WrapError(core.ErrStoreFailure, "feedback missing")
	}
	return feedback, nil
}

func (s *InMemoryFeedbackStore) ListFeedback(tenantID string) ([]FeedbackRecord, error) {
	if err := validateSafeID("tenant_id", tenantID); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	records := make([]FeedbackRecord, 0)
	for _, record := range s.records {
		if record.TenantID == tenantID {
			records = append(records, record)
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].ID < records[j].ID
	})
	return records, nil
}

func validateFeedbackRecord(record FeedbackRecord) error {
	for field, value := range map[string]string{
		"feedback_id":     record.ID,
		"tenant_id":       record.TenantID,
		"conversation_id": record.ConversationID,
		"message_id":      record.MessageID,
		"created_by":      record.CreatedBy,
	} {
		if err := validateSafeID(field, value); err != nil {
			return err
		}
	}
	if record.Note == "" {
		return validateSafeID("note", "")
	}
	if !validFeedbackCategory(record.Category) || !validFeedbackSeverity(record.Severity) || !validFeedbackStatus(record.Status) {
		return validateSafeID("feedback_enum", "")
	}
	if record.LinkedEvalCaseID != "" {
		if err := validateSafeID("linked_eval_case_id", record.LinkedEvalCaseID); err != nil {
			return err
		}
	}
	return nil
}

func validFeedbackCategory(value FeedbackCategory) bool {
	switch value {
	case FeedbackPersona, FeedbackKnowledge, FeedbackTool, FeedbackSafety:
		return true
	default:
		return false
	}
}

func validFeedbackSeverity(value FeedbackSeverity) bool {
	switch value {
	case FeedbackSeverityLow, FeedbackSeverityMedium, FeedbackSeverityHigh:
		return true
	default:
		return false
	}
}

func validFeedbackStatus(value FeedbackStatus) bool {
	switch value {
	case FeedbackNew, FeedbackTriaged, FeedbackEvalAdded, FeedbackKnowledgeFixNeeded, FeedbackPersonaFixNeeded, FeedbackDismissed, FeedbackResolved:
		return true
	default:
		return false
	}
}

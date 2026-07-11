package admin

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

type RepairRecurrenceProjectionState string

const (
	RepairRecurrenceProjectionNone      RepairRecurrenceProjectionState = "none"
	RepairRecurrenceProjectionSuspected RepairRecurrenceProjectionState = "suspected"
	RepairRecurrenceProjectionDismissed RepairRecurrenceProjectionState = "dismissed"
	RepairRecurrenceProjectionConfirmed RepairRecurrenceProjectionState = "confirmed"
)

type RepairRecurrenceProjection struct {
	State          RepairRecurrenceProjectionState
	Count          int
	LatestAt       *time.Time
	LatestAnswer   string
	LatestReason   string
	LatestRecordID string
}

type RepairRecurrenceService struct {
	store        RepairRecurrenceStore
	gaps         KnowledgeGapService
	verification RepairVerificationService
}

var (
	ErrRepairRecurrenceUnavailable   = errors.New("repair recurrence service unavailable")
	ErrRepairRecurrenceInvalidStatus = errors.New("invalid recurrence status")
)

func NewRepairRecurrenceService(store RepairRecurrenceStore, gaps KnowledgeGapService, verification RepairVerificationService) RepairRecurrenceService {
	return RepairRecurrenceService{store: store, gaps: gaps, verification: verification}
}

func (s RepairRecurrenceService) Detect(tenantID string, audit AuditRecord) (RepairRecurrence, bool, error) {
	if s.store == nil {
		return RepairRecurrence{}, false, ErrRepairRecurrenceUnavailable
	}
	if !isWeakRecurrenceAnswer(audit.KnowledgeAnswerState) || strings.TrimSpace(audit.ID) == "" || strings.TrimSpace(audit.KnowledgeSpaceID) == "" || strings.TrimSpace(audit.QuestionSummary) == "" || strings.TrimSpace(audit.KnowledgeNoSourceReason) == "" {
		return RepairRecurrence{}, false, nil
	}
	gap, matchedBy, ok, err := s.findEligibleGap(tenantID, audit)
	if err != nil || !ok {
		return RepairRecurrence{}, false, err
	}
	projection, err := s.verification.CurrentProjection(tenantID, gap)
	if err != nil || projection.State != RepairVerificationVerified {
		return RepairRecurrence{}, false, err
	}
	now := audit.CreatedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	candidate := RepairRecurrence{
		ID: fmt.Sprintf("recurrence-%s", audit.ID), TenantID: tenantID, GapID: gap.ID, SpaceID: gap.SpaceID,
		Status: RepairRecurrenceSuspected, MatchedBy: matchedBy, FirstAuditID: audit.ID, LatestAuditID: audit.ID,
		OccurrenceCount: 1, AnswerState: strings.TrimSpace(audit.KnowledgeAnswerState), NoSourceReason: strings.TrimSpace(audit.KnowledgeNoSourceReason),
		VerifiedAttemptID: projection.VerifiedAttemptID, VerifiedSnapshotFingerprint: projection.VerifiedSnapshotFingerprint, CreatedAt: now, UpdatedAt: now,
	}
	record, _, err := s.store.ObserveRepairRecurrence(RepairRecurrenceObservation{TenantID: tenantID, GapID: gap.ID, AuditID: audit.ID, RecordedAt: now}, candidate)
	if err != nil {
		return RepairRecurrence{}, false, err
	}
	return record, true, nil
}

func (s RepairRecurrenceService) findEligibleGap(tenantID string, audit AuditRecord) (KnowledgeGap, RepairRecurrenceMatch, bool, error) {
	if explicitID := explicitAuditGapID(audit.KnowledgeEvidence); explicitID != "" {
		gap, err := s.gaps.Get(tenantID, explicitID)
		if err != nil || gap.Status != KnowledgeGapResolved || gap.SpaceID != strings.TrimSpace(audit.KnowledgeSpaceID) {
			return KnowledgeGap{}, RepairRecurrenceMatchExplicit, false, nil
		}
		projection, err := s.verification.CurrentProjection(tenantID, gap)
		if err != nil {
			return KnowledgeGap{}, RepairRecurrenceMatchExplicit, false, err
		}
		return gap, RepairRecurrenceMatchExplicit, projection.State == RepairVerificationVerified, nil
	}
	gaps, err := s.gaps.List(tenantID, audit.KnowledgeSpaceID)
	if err != nil {
		return KnowledgeGap{}, RepairRecurrenceMatchExact, false, err
	}
	var matches []KnowledgeGap
	for _, gap := range gaps {
		if gap.Status != KnowledgeGapResolved || normalizeRecurrenceText(gap.Question) != normalizeRecurrenceText(audit.QuestionSummary) || strings.TrimSpace(gap.NoSourceReason) != strings.TrimSpace(audit.KnowledgeNoSourceReason) {
			continue
		}
		projection, projectionErr := s.verification.CurrentProjection(tenantID, gap)
		if projectionErr != nil {
			return KnowledgeGap{}, RepairRecurrenceMatchExact, false, projectionErr
		}
		if projection.State == RepairVerificationVerified {
			matches = append(matches, gap)
		}
	}
	if len(matches) != 1 {
		return KnowledgeGap{}, RepairRecurrenceMatchExact, false, nil
	}
	return matches[0], RepairRecurrenceMatchExact, true, nil
}

func (s RepairRecurrenceService) List(tenantID, gapID, status string, limit int) ([]RepairRecurrence, error) {
	if s.store == nil {
		return nil, ErrRepairRecurrenceUnavailable
	}
	if status != "" && status != string(RepairRecurrenceSuspected) && status != string(RepairRecurrenceDismissed) && status != string(RepairRecurrenceConfirmed) {
		return nil, ErrRepairRecurrenceInvalidStatus
	}
	return s.store.ListRepairRecurrences(tenantID, gapID, status, limit)
}

func (s RepairRecurrenceService) Project(tenantID, gapID string) (RepairRecurrenceProjection, error) {
	items, err := s.List(tenantID, gapID, "", 100)
	if err != nil {
		return RepairRecurrenceProjection{}, err
	}
	projection := RepairRecurrenceProjection{State: RepairRecurrenceProjectionNone}
	if len(items) == 0 {
		return projection, nil
	}
	for _, item := range items {
		if item.Status == RepairRecurrenceSuspected {
			at := item.UpdatedAt
			return RepairRecurrenceProjection{State: RepairRecurrenceProjectionSuspected, Count: item.OccurrenceCount, LatestAt: &at, LatestAnswer: item.AnswerState, LatestReason: item.NoSourceReason, LatestRecordID: item.ID}, nil
		}
	}
	latest := items[0]
	state := RepairRecurrenceProjectionDismissed
	if latest.Status == RepairRecurrenceConfirmed {
		state = RepairRecurrenceProjectionConfirmed
	}
	at := latest.UpdatedAt
	return RepairRecurrenceProjection{State: state, Count: latest.OccurrenceCount, LatestAt: &at, LatestAnswer: latest.AnswerState, LatestReason: latest.NoSourceReason, LatestRecordID: latest.ID}, nil
}

func (s RepairRecurrenceService) Confirm(tenantID, recurrenceID, confirmedBy string) (RepairRecurrence, error) {
	record, err := s.store.GetRepairRecurrence(tenantID, recurrenceID)
	if err != nil {
		return RepairRecurrence{}, err
	}
	if record.Status != RepairRecurrenceSuspected {
		return RepairRecurrence{}, errors.New("recurrence is not pending")
	}
	gap, err := s.gaps.Get(tenantID, record.GapID)
	if err != nil {
		return RepairRecurrence{}, err
	}
	if gap.Status != KnowledgeGapOpen {
		if _, err := s.gaps.UpdateStatus(tenantID, gap.ID, KnowledgeGapOpen, "", ""); err != nil {
			return RepairRecurrence{}, err
		}
	}
	now := time.Now().UTC()
	record.Status = RepairRecurrenceConfirmed
	record.ConfirmedAt = &now
	record.ConfirmedBy = normalizeRecurrenceText(confirmedBy)
	record.UpdatedAt = now
	return s.store.SaveRepairRecurrence(record)
}

func (s RepairRecurrenceService) Dismiss(tenantID, recurrenceID, reason string) (RepairRecurrence, error) {
	record, err := s.store.GetRepairRecurrence(tenantID, recurrenceID)
	if err != nil {
		return RepairRecurrence{}, err
	}
	if record.Status != RepairRecurrenceSuspected {
		return RepairRecurrence{}, errors.New("recurrence is not pending")
	}
	reason = normalizeRecurrenceText(reason)
	if reason == "" || utf8.RuneCountInString(reason) > 240 {
		return RepairRecurrence{}, errors.New("dismiss reason is required and must be at most 240 characters")
	}
	now := time.Now().UTC()
	record.Status = RepairRecurrenceDismissed
	record.DismissedAt = &now
	record.DismissReason = reason
	record.UpdatedAt = now
	return s.store.SaveRepairRecurrence(record)
}

func isWeakRecurrenceAnswer(state string) bool {
	switch strings.TrimSpace(state) {
	case "unsupported", "partially_supported", "review_gated":
		return true
	default:
		return false
	}
}

func explicitAuditGapID(evidence map[string]any) string {
	switch items := evidence["gaps"].(type) {
	case []map[string]any:
		if len(items) == 0 {
			return ""
		}
		gapID, _ := items[0]["gap_id"].(string)
		return strings.TrimSpace(gapID)
	case []any:
		for _, value := range items {
			if item, ok := value.(map[string]any); ok {
				if gapID, ok := item["gap_id"].(string); ok {
					return strings.TrimSpace(gapID)
				}
			}
		}
	}
	return ""
}

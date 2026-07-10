package admin

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type KnowledgeRepairPriority string

const (
	KnowledgeRepairPriorityHigh   KnowledgeRepairPriority = "high"
	KnowledgeRepairPriorityMedium KnowledgeRepairPriority = "medium"
	KnowledgeRepairPriorityLow    KnowledgeRepairPriority = "low"
)

type KnowledgeRepairFilter struct {
	SpaceID        string
	Status         string
	Reason         string
	WeakOnly       bool
	UnresolvedOnly bool
	LinkedEvidence bool
	Limit          int
}

type KnowledgeRepairLinkedDocument struct {
	DocumentID   string `json:"document_id"`
	Name         string `json:"name,omitempty"`
	ReviewStatus string `json:"review_status,omitempty"`
	SourceType   string `json:"source_type,omitempty"`
	Status       string `json:"status,omitempty"`
	Relation     string `json:"relation,omitempty"`
}

type KnowledgeRepairItem struct {
	RepairID                string                          `json:"repair_id"`
	GapID                   string                          `json:"gap_id"`
	SpaceID                 string                          `json:"space_id"`
	SpaceName               string                          `json:"space_name,omitempty"`
	QuestionSummary         string                          `json:"question_summary"`
	Status                  KnowledgeGapStatus              `json:"status"`
	AnswerState             string                          `json:"answer_state"`
	Reason                  string                          `json:"reason"`
	Priority                KnowledgeRepairPriority         `json:"priority"`
	PriorityReasons         []string                        `json:"priority_reasons,omitempty"`
	LastSeenAt              time.Time                       `json:"last_seen_at"`
	OccurrenceCount         int                             `json:"occurrence_count"`
	SourceCount             int                             `json:"source_count"`
	LinkedDocuments         []KnowledgeRepairLinkedDocument `json:"linked_documents"`
	NextAction              string                          `json:"next_action"`
	VerificationState       RepairVerificationState         `json:"verification_state"`
	LastVerifiedAt          *time.Time                      `json:"last_verified_at,omitempty"`
	LastVerificationResult  RepairVerificationResult        `json:"last_verification_result,omitempty"`
	LastVerificationFailure RepairVerificationFailureReason `json:"last_verification_failure_reason,omitempty"`
}

type KnowledgeRepairService struct {
	gaps         KnowledgeGapService
	audit        AuditService
	knowledge    KnowledgeService
	verification *RepairVerificationService
}

func NewKnowledgeRepairService(gaps KnowledgeGapService, audit AuditService, knowledge KnowledgeService, verification ...*RepairVerificationService) KnowledgeRepairService {
	service := KnowledgeRepairService{
		gaps:      gaps,
		audit:     audit,
		knowledge: knowledge,
	}
	if len(verification) > 0 {
		service.verification = verification[0]
	}
	return service
}

func (s KnowledgeRepairService) List(tenantID string, filter KnowledgeRepairFilter) ([]KnowledgeRepairItem, error) {
	spaceID := normalizeDocumentSpaceID(filter.SpaceID)
	gaps, err := s.gaps.List(tenantID, spaceID)
	if err != nil {
		return nil, err
	}
	timeline, err := s.audit.Timeline(tenantID, AnswerAuditTimelineFilter{})
	if err != nil {
		return nil, err
	}
	documents, err := s.knowledge.ListBySpace(tenantID, spaceID)
	if err != nil {
		return nil, err
	}
	spaceName := spaceID
	if space, err := s.knowledge.GetSpace(tenantID, spaceID); err == nil && strings.TrimSpace(space.Name) != "" {
		spaceName = space.Name
	}

	items := make([]KnowledgeRepairItem, 0, len(gaps))
	for _, gap := range gaps {
		item := s.projectRepairItem(gap, spaceName, timeline, documents)
		if s.verification != nil {
			projection, err := s.verification.Project(tenantID, gap, documents)
			if err != nil {
				return nil, err
			}
			item.VerificationState = projection.State
			item.LastVerifiedAt = projection.LastVerifiedAt
			item.LastVerificationResult = projection.LastResult
			item.LastVerificationFailure = projection.LastFailure
		} else {
			item.VerificationState = RepairVerificationUnverified
		}
		if !matchesRepairFilter(item, filter) {
			continue
		}
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		return repairSortLess(items[i], items[j])
	})
	if filter.Limit > 0 && len(items) > filter.Limit {
		items = items[:filter.Limit]
	}
	return items, nil
}

func (s KnowledgeRepairService) projectRepairItem(gap KnowledgeGap, spaceName string, timeline []AnswerAuditTimelineItem, documents []KnowledgeDocument) KnowledgeRepairItem {
	matches := matchingTimelineItems(gap, timeline)
	linked := linkedRepairDocuments(gap, documents)
	answerState := "unknown"
	sourceCount := 0
	lastSeenAt := gap.UpdatedAt
	occurrenceCount := len(matches)
	if len(matches) > 0 {
		answerState = matches[0].AnswerState
		sourceCount = matches[0].SourceCount
		lastSeenAt = matches[0].CreatedAt
	}
	priorityReasons := repairPriorityReasons(gap, answerState, occurrenceCount, len(linked))
	priority := repairPriorityFor(gap, answerState, occurrenceCount, len(linked))
	return KnowledgeRepairItem{
		RepairID:        fmt.Sprintf("repair-%s", gap.ID),
		GapID:           gap.ID,
		SpaceID:         gap.SpaceID,
		SpaceName:       spaceName,
		QuestionSummary: gap.Question,
		Status:          gap.Status,
		AnswerState:     answerState,
		Reason:          gap.NoSourceReason,
		Priority:        priority,
		PriorityReasons: priorityReasons,
		LastSeenAt:      lastSeenAt,
		OccurrenceCount: occurrenceCount,
		SourceCount:     sourceCount,
		LinkedDocuments: linked,
		NextAction:      repairNextAction(gap, answerState, len(linked)),
	}
}

func matchingTimelineItems(gap KnowledgeGap, timeline []AnswerAuditTimelineItem) []AnswerAuditTimelineItem {
	matches := make([]AnswerAuditTimelineItem, 0, 4)
	for _, item := range timeline {
		if item.Gap != nil && strings.TrimSpace(item.Gap.GapID) == gap.ID {
			matches = append(matches, item)
			continue
		}
		if strings.TrimSpace(item.KnowledgeSpaceID) != gap.SpaceID {
			continue
		}
		if strings.TrimSpace(item.QuestionSummary) != strings.TrimSpace(gap.Question) {
			continue
		}
		if strings.TrimSpace(item.Diagnostics.NoSourceReason) != strings.TrimSpace(gap.NoSourceReason) {
			continue
		}
		matches = append(matches, item)
	}
	return matches
}

func linkedRepairDocuments(gap KnowledgeGap, documents []KnowledgeDocument) []KnowledgeRepairLinkedDocument {
	linked := make([]KnowledgeRepairLinkedDocument, 0, 4)
	added := make(map[string]struct{})
	appendLinked := func(document KnowledgeDocument, relation string) {
		if _, ok := added[document.ID]; ok {
			return
		}
		added[document.ID] = struct{}{}
		linked = append(linked, KnowledgeRepairLinkedDocument{
			DocumentID:   document.ID,
			Name:         document.Name,
			ReviewStatus: string(effectiveKnowledgeReviewStatus(document)),
			SourceType:   strings.TrimSpace(document.Metadata["source_type"]),
			Status:       string(document.Status),
			Relation:     relation,
		})
	}
	for _, document := range documents {
		if document.ID == gap.ResolvedByDocumentID {
			appendLinked(document, "resolved_by")
			continue
		}
		if strings.TrimSpace(document.Metadata["source_gap_id"]) == gap.ID {
			appendLinked(document, "source_gap")
		}
	}
	sort.SliceStable(linked, func(i, j int) bool {
		if linked[i].Relation != linked[j].Relation {
			return linked[i].Relation < linked[j].Relation
		}
		return linked[i].DocumentID < linked[j].DocumentID
	})
	return linked
}

func repairPriorityReasons(gap KnowledgeGap, answerState string, occurrenceCount int, linkedCount int) []string {
	reasons := make([]string, 0, 6)
	switch gap.Status {
	case KnowledgeGapOpen, KnowledgeGapInvestigating:
		reasons = append(reasons, "unresolved")
	case KnowledgeGapResolved:
		reasons = append(reasons, "resolved")
	case KnowledgeGapIgnored:
		reasons = append(reasons, "ignored")
	}
	if occurrenceCount > 1 {
		reasons = append(reasons, "repeated_weak_answer")
	}
	if linkedCount == 0 {
		reasons = append(reasons, "no_linked_evidence")
	} else {
		reasons = append(reasons, "linked_evidence_present")
	}
	if strings.TrimSpace(answerState) == "partially_supported" {
		reasons = append(reasons, "partially_supported")
	}
	if strings.TrimSpace(answerState) == "review_gated" || strings.TrimSpace(gap.NoSourceReason) == "review_gated_documents" {
		reasons = append(reasons, "review_gated")
	}
	if occurrenceCount == 0 {
		reasons = append(reasons, "no_timeline_context")
	}
	return reasons
}

func repairPriorityFor(gap KnowledgeGap, answerState string, occurrenceCount int, linkedCount int) KnowledgeRepairPriority {
	if gap.Status == KnowledgeGapResolved || gap.Status == KnowledgeGapIgnored {
		return KnowledgeRepairPriorityLow
	}
	if occurrenceCount > 1 || linkedCount == 0 || strings.TrimSpace(answerState) == "review_gated" || strings.TrimSpace(gap.NoSourceReason) == "review_gated_documents" {
		return KnowledgeRepairPriorityHigh
	}
	return KnowledgeRepairPriorityMedium
}

func repairNextAction(gap KnowledgeGap, answerState string, linkedCount int) string {
	switch gap.Status {
	case KnowledgeGapResolved:
		return "Repair recorded. Reopen if weak answers continue."
	case KnowledgeGapIgnored:
		return "Ignored for now. Reopen if the question becomes important."
	}
	if strings.TrimSpace(answerState) == "review_gated" || strings.TrimSpace(gap.NoSourceReason) == "review_gated_documents" {
		return "Review or activate a linked source, then retest."
	}
	if linkedCount == 0 {
		return "Run diagnostics or create a reviewed note."
	}
	return "Inspect linked evidence and retest the original question."
}

func matchesRepairFilter(item KnowledgeRepairItem, filter KnowledgeRepairFilter) bool {
	if strings.TrimSpace(filter.Status) != "" && string(item.Status) != strings.TrimSpace(filter.Status) {
		return false
	}
	if strings.TrimSpace(filter.Reason) != "" && item.Reason != strings.TrimSpace(filter.Reason) {
		return false
	}
	if filter.UnresolvedOnly && item.Status != KnowledgeGapOpen && item.Status != KnowledgeGapInvestigating {
		return false
	}
	if filter.LinkedEvidence && len(item.LinkedDocuments) == 0 {
		return false
	}
	if filter.WeakOnly && !repairWeakState(item.AnswerState) && strings.TrimSpace(item.Reason) == "" {
		return false
	}
	return true
}

func repairWeakState(state string) bool {
	switch strings.TrimSpace(state) {
	case "unsupported", "partially_supported", "review_gated", "provider_fallback", "guard_rejected":
		return true
	default:
		return false
	}
}

func repairSortLess(left, right KnowledgeRepairItem) bool {
	if repairStatusRank(left.Status) != repairStatusRank(right.Status) {
		return repairStatusRank(left.Status) < repairStatusRank(right.Status)
	}
	if repairPriorityRank(left.Priority) != repairPriorityRank(right.Priority) {
		return repairPriorityRank(left.Priority) < repairPriorityRank(right.Priority)
	}
	if left.OccurrenceCount != right.OccurrenceCount {
		return left.OccurrenceCount > right.OccurrenceCount
	}
	if !left.LastSeenAt.Equal(right.LastSeenAt) {
		return left.LastSeenAt.After(right.LastSeenAt)
	}
	return left.GapID < right.GapID
}

func repairStatusRank(status KnowledgeGapStatus) int {
	switch status {
	case KnowledgeGapOpen, KnowledgeGapInvestigating:
		return 0
	case KnowledgeGapResolved, KnowledgeGapIgnored:
		return 1
	default:
		return 2
	}
}

func repairPriorityRank(priority KnowledgeRepairPriority) int {
	switch priority {
	case KnowledgeRepairPriorityHigh:
		return 0
	case KnowledgeRepairPriorityMedium:
		return 1
	case KnowledgeRepairPriorityLow:
		return 2
	default:
		return 3
	}
}

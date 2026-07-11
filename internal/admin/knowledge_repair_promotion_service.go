package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var (
	ErrRepairEvalPromotionInvalidRequest      = errors.New("invalid repair eval promotion request")
	ErrRepairEvalPromotionUnavailable         = errors.New("repair eval promotion service unavailable")
	ErrRepairEvalPromotionGapNotEligible      = errors.New("knowledge gap is not eligible for repair eval promotion")
	ErrRepairEvalPromotionVerificationStale   = errors.New("repair verification is stale or does not match the requested attempt")
	ErrRepairEvalPromotionRecurrencePending   = errors.New("repair recurrence requires review before promotion")
	ErrRepairEvalPromotionDocumentNotEligible = errors.New("required document is not eligible for repair eval promotion")
)

type RepairEvalPromotionProjectionState string

const (
	RepairEvalPromotionNotPromoted RepairEvalPromotionProjectionState = "not_promoted"
	RepairEvalPromotionPromoted    RepairEvalPromotionProjectionState = "promoted"
	RepairEvalPromotionStale       RepairEvalPromotionProjectionState = "promotion_stale"
)

type RepairEvalPromotionRequest struct {
	GapID                 string
	VerificationAttemptID string
	MinimumSupportState   RepairEvalSupportState
	RequiredDocumentIDs   []string
	PromotedBy            string
}

type RepairEvalPromotionService struct {
	store        RepairEvalPromotionStore
	gaps         KnowledgeGapService
	knowledge    KnowledgeService
	verification RepairVerificationService
	recurrence   *RepairRecurrenceService
	now          func() time.Time
}

func NewRepairEvalPromotionService(store RepairEvalPromotionStore, gaps KnowledgeGapService, knowledge KnowledgeService, verification RepairVerificationService, recurrence *RepairRecurrenceService) RepairEvalPromotionService {
	return RepairEvalPromotionService{store: store, gaps: gaps, knowledge: knowledge, verification: verification, recurrence: recurrence, now: func() time.Time { return time.Now().UTC() }}
}

func (s RepairEvalPromotionService) Promote(ctx context.Context, tenantID string, request RepairEvalPromotionRequest) (RepairEvalPromotionRevision, bool, error) {
	_ = ctx
	if s.store == nil {
		return RepairEvalPromotionRevision{}, false, ErrRepairEvalPromotionUnavailable
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" || strings.TrimSpace(request.GapID) == "" || strings.TrimSpace(request.VerificationAttemptID) == "" || strings.TrimSpace(request.PromotedBy) == "" {
		return RepairEvalPromotionRevision{}, false, ErrRepairEvalPromotionInvalidRequest
	}
	for _, value := range []string{tenantID, strings.TrimSpace(request.GapID), strings.TrimSpace(request.VerificationAttemptID), strings.TrimSpace(request.PromotedBy)} {
		if err := validateKnowledgeID(value); err != nil {
			return RepairEvalPromotionRevision{}, false, ErrRepairEvalPromotionInvalidRequest
		}
	}
	if request.MinimumSupportState != RepairEvalSupportPartiallySupported && request.MinimumSupportState != RepairEvalSupportGrounded {
		return RepairEvalPromotionRevision{}, false, ErrRepairEvalPromotionInvalidRequest
	}
	gap, err := s.gaps.Get(tenantID, request.GapID)
	if err != nil {
		return RepairEvalPromotionRevision{}, false, err
	}
	if gap.Status != KnowledgeGapResolved {
		return RepairEvalPromotionRevision{}, false, ErrRepairEvalPromotionGapNotEligible
	}
	projection, err := s.verification.CurrentProjection(tenantID, gap)
	if err != nil {
		return RepairEvalPromotionRevision{}, false, err
	}
	if projection.State != RepairVerificationVerified || projection.VerifiedAttemptID != strings.TrimSpace(request.VerificationAttemptID) || strings.TrimSpace(projection.VerifiedSnapshotFingerprint) == "" {
		return RepairEvalPromotionRevision{}, false, ErrRepairEvalPromotionVerificationStale
	}
	if s.recurrence != nil {
		recurrence, err := s.recurrence.Project(tenantID, gap.ID)
		if err != nil {
			return RepairEvalPromotionRevision{}, false, err
		}
		if recurrence.State == RepairRecurrenceProjectionSuspected {
			return RepairEvalPromotionRevision{}, false, ErrRepairEvalPromotionRecurrencePending
		}
	}
	documentIDs, err := s.validateRequiredDocuments(tenantID, gap, request.RequiredDocumentIDs)
	if err != nil {
		return RepairEvalPromotionRevision{}, false, err
	}
	policyFingerprint, err := repairEvalPolicyFingerprint(request.MinimumSupportState, documentIDs)
	if err != nil {
		return RepairEvalPromotionRevision{}, false, err
	}
	candidate := RepairEvalPromotionRevision{
		TenantID: tenantID, CaseID: fmt.Sprintf("repair-eval-%s-%s", tenantID, gap.ID), GapID: gap.ID, SpaceID: gap.SpaceID,
		Question: gap.Question, VerificationAttemptID: projection.VerifiedAttemptID, VerificationSnapshotFingerprint: projection.VerifiedSnapshotFingerprint,
		MinimumSupportState: request.MinimumSupportState, RequiredDocumentIDs: documentIDs, PolicyFingerprint: policyFingerprint,
		PromotedBy: strings.TrimSpace(request.PromotedBy), PromotedAt: s.now(),
	}
	return s.store.PromoteRepairEval(candidate)
}

func (s RepairEvalPromotionService) validateRequiredDocuments(tenantID string, gap KnowledgeGap, ids []string) ([]string, error) {
	seen := make(map[string]struct{}, len(ids))
	validated := make([]string, 0, len(ids))
	for _, rawID := range ids {
		id := strings.TrimSpace(rawID)
		if id == "" {
			return nil, ErrRepairEvalPromotionInvalidRequest
		}
		if _, ok := seen[id]; ok {
			return nil, ErrRepairEvalPromotionInvalidRequest
		}
		seen[id] = struct{}{}
		document, err := s.knowledge.Get(tenantID, id)
		if err != nil {
			return nil, ErrRepairEvalPromotionDocumentNotEligible
		}
		if normalizeDocumentSpaceID(document.SpaceID) != normalizeDocumentSpaceID(gap.SpaceID) || document.Status != KnowledgeReady || effectiveKnowledgeReviewStatus(document) != KnowledgeReviewActive {
			return nil, ErrRepairEvalPromotionDocumentNotEligible
		}
		validated = append(validated, id)
	}
	sort.Strings(validated)
	return validated, nil
}

func repairEvalPolicyFingerprint(state RepairEvalSupportState, documentIDs []string) (string, error) {
	payload := struct {
		MinimumSupportState RepairEvalSupportState `json:"minimum_support_state"`
		RequiredDocumentIDs []string               `json:"required_document_ids"`
	}{MinimumSupportState: state, RequiredDocumentIDs: append([]string(nil), documentIDs...)}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(encoded)
	return "sha256-" + hex.EncodeToString(hash[:]), nil
}

package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const qualityReviewCandidateLimit = 10

const (
	qualityReviewSchemaVersion = 1
	qualityReviewRationaleMax  = 500
	qualityReviewKeyMinLength  = 16
	qualityReviewKeyMaxLength  = 128
)

var (
	ErrQualityReviewInvalid             = errors.New("invalid quality review checkpoint")
	ErrQualityReviewIdempotencyConflict = errors.New("quality review idempotency conflict")
	ErrQualityReviewGapNotEligible      = errors.New("quality review gap not eligible")
	ErrQualityReviewUnavailable         = errors.New("quality review unavailable")
	ErrQualityReviewCapacity            = errors.New("quality review capacity reached")
)

type QualityReviewOutcome string

const (
	QualityReviewObserve        QualityReviewOutcome = "observe"
	QualityReviewRepairRequired QualityReviewOutcome = "repair_required"
	QualityReviewRiskAccepted   QualityReviewOutcome = "risk_accepted"
)

type QualityReviewMetricSnapshotV1 struct {
	Status        string   `json:"status"`
	Count         int      `json:"count"`
	Denominator   int      `json:"denominator,omitempty"`
	Value         *float64 `json:"value,omitempty"`
	MedianMS      *int64   `json:"median_ms,omitempty"`
	ExcludedCount int      `json:"excluded_count,omitempty"`
}

type QualityReviewVerificationSnapshotV1 struct {
	AttemptCount   int                           `json:"attempt_count"`
	PassedGapCount int                           `json:"passed_gap_count"`
	TimeToVerify   QualityReviewMetricSnapshotV1 `json:"time_to_verify"`
	CurrentStale   QualityReviewMetricSnapshotV1 `json:"current_stale"`
}

type QualityReviewRecurrenceSnapshotV1 struct {
	SuspectedCount int                           `json:"suspected_count"`
	ConfirmedCount int                           `json:"confirmed_count"`
	DismissedCount int                           `json:"dismissed_count"`
	Rate           QualityReviewMetricSnapshotV1 `json:"rate"`
}

type QualityReviewPromotedEvalSnapshotV1 struct {
	PassedCount      int                           `json:"passed_count"`
	FailedCount      int                           `json:"failed_count"`
	UnavailableCount int                           `json:"unavailable_count"`
	Rate             QualityReviewMetricSnapshotV1 `json:"rate"`
}

type QualityReviewSnapshotV1 struct {
	ProjectedAt  time.Time                           `json:"projected_at"`
	Verification QualityReviewVerificationSnapshotV1 `json:"verification"`
	Recurrence   QualityReviewRecurrenceSnapshotV1   `json:"recurrence"`
	PromotedEval QualityReviewPromotedEvalSnapshotV1 `json:"promoted_eval"`
}

type QualityReviewFilter struct {
	From       string    `json:"from"`
	To         string    `json:"to"`
	SpaceID    string    `json:"space_id,omitempty"`
	AllSpaces  bool      `json:"all_spaces"`
	Timezone   string    `json:"timezone"`
	WindowDays int       `json:"window_days"`
	Start      time.Time `json:"start_at"`
	End        time.Time `json:"end_exclusive_at"`
}

type QualityReviewCheckpoint struct {
	SchemaVersion      int                     `json:"schema_version"`
	ID                 string                  `json:"id"`
	TenantID           string                  `json:"tenant_id"`
	CreatedAt          time.Time               `json:"created_at"`
	Filter             QualityReviewFilter     `json:"filter"`
	Outcome            QualityReviewOutcome    `json:"outcome"`
	Rationale          string                  `json:"rationale,omitempty"`
	GapIDs             []string                `json:"gap_ids,omitempty"`
	Snapshot           QualityReviewSnapshotV1 `json:"snapshot"`
	IdempotencyKeyHash string                  `json:"idempotency_key_hash"`
	RequestFingerprint string                  `json:"request_fingerprint"`
}

type QualityReviewCheckpointRequest struct {
	IdempotencyKey string               `json:"idempotency_key"`
	From           string               `json:"from"`
	To             string               `json:"to"`
	SpaceID        string               `json:"space_id,omitempty"`
	Outcome        QualityReviewOutcome `json:"outcome"`
	Rationale      string               `json:"rationale,omitempty"`
	GapIDs         []string             `json:"gap_ids,omitempty"`
}

type QualityReviewCheckpointCreateResult struct {
	Checkpoint QualityReviewCheckpoint `json:"checkpoint"`
	Created    bool                    `json:"created"`
}

type QualityReviewCheckpointHistory struct {
	Checkpoints     []QualityReviewCheckpoint `json:"checkpoints"`
	ExcludedRecords int                       `json:"excluded_records,omitempty"`
	Comparison      *QualityReviewComparison  `json:"comparison,omitempty"`
}

// QualityReviewComparison is deliberately descriptive: a difference is not an improvement claim.
type QualityReviewComparison struct {
	CurrentID   string                          `json:"current_id"`
	PreviousID  string                          `json:"previous_id"`
	Current     QualityReviewFilter             `json:"current"`
	Previous    QualityReviewFilter             `json:"previous"`
	Differences []QualityReviewMetricDifference `json:"differences"`
}

type QualityReviewMetricDifference struct {
	Metric   string  `json:"metric"`
	Status   string  `json:"status"`
	Current  float64 `json:"current,omitempty"`
	Previous float64 `json:"previous,omitempty"`
	Delta    float64 `json:"delta,omitempty"`
	Unit     string  `json:"unit,omitempty"`
}

type QualityTrendProjector interface {
	Project(tenantID string, request QualityTrendRequest) (QualityTrendProjection, error)
}

type QualityReviewGapReader interface {
	Get(tenantID, gapID string) (KnowledgeGap, error)
}

type QualityReviewCheckpointStore interface {
	FindQualityReviewCheckpointByIdempotency(tenantID, keyHash string) (QualityReviewCheckpoint, bool, error)
	AppendQualityReviewCheckpoint(record QualityReviewCheckpoint) (QualityReviewCheckpoint, bool, error)
	ListQualityReviewCheckpoints(tenantID string) ([]QualityReviewCheckpoint, int, error)
}

type QualityReviewCheckpointDependencies struct {
	Trends   QualityTrendProjector
	Gaps     QualityReviewGapReader
	Store    QualityReviewCheckpointStore
	Now      func() time.Time
	NewID    func() (string, error)
	Location *time.Location
}

type QualityReviewCheckpointService struct {
	trends   QualityTrendProjector
	gaps     QualityReviewGapReader
	store    QualityReviewCheckpointStore
	now      func() time.Time
	newID    func() (string, error)
	location *time.Location
}

func NewQualityReviewCheckpointService(dependencies QualityReviewCheckpointDependencies) QualityReviewCheckpointService {
	now := dependencies.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	newID := dependencies.NewID
	if newID == nil {
		newID = newQualityReviewCheckpointID
	}
	location := dependencies.Location
	if location == nil {
		location = time.Local
	}
	return QualityReviewCheckpointService{
		trends: dependencies.Trends, gaps: dependencies.Gaps, store: dependencies.Store,
		now: now, newID: newID, location: location,
	}
}

func (s QualityReviewCheckpointService) Create(tenantID string, request QualityReviewCheckpointRequest) (QualityReviewCheckpointCreateResult, error) {
	if s.trends == nil || s.gaps == nil || s.store == nil || strings.TrimSpace(tenantID) == "" {
		return QualityReviewCheckpointCreateResult{}, ErrQualityReviewUnavailable
	}
	now := s.now()
	normalized, err := normalizeQualityReviewCheckpointRequest(request, now, s.location)
	if err != nil {
		return QualityReviewCheckpointCreateResult{}, err
	}
	keyHash := qualityReviewIdempotencyKeyHash(tenantID, normalized.IdempotencyKey)
	fingerprint, err := qualityReviewRequestFingerprint(tenantID, normalized)
	if err != nil {
		return QualityReviewCheckpointCreateResult{}, fmt.Errorf("%w: fingerprint", ErrQualityReviewInvalid)
	}
	existing, found, err := s.store.FindQualityReviewCheckpointByIdempotency(tenantID, keyHash)
	if err != nil {
		return QualityReviewCheckpointCreateResult{}, fmt.Errorf("%w: %v", ErrQualityReviewUnavailable, err)
	}
	if found {
		if existing.RequestFingerprint != fingerprint {
			return QualityReviewCheckpointCreateResult{}, ErrQualityReviewIdempotencyConflict
		}
		return QualityReviewCheckpointCreateResult{Checkpoint: cloneQualityReviewCheckpoint(existing)}, nil
	}

	projection, err := s.trends.Project(tenantID, QualityTrendRequest{From: normalized.From, To: normalized.To, SpaceID: normalized.SpaceID})
	if err != nil {
		return QualityReviewCheckpointCreateResult{}, fmt.Errorf("%w: %v", ErrQualityReviewUnavailable, err)
	}
	if err := s.validateSelectedGaps(tenantID, normalized, projection); err != nil {
		return QualityReviewCheckpointCreateResult{}, err
	}
	id, err := s.newID()
	if err != nil {
		return QualityReviewCheckpointCreateResult{}, fmt.Errorf("%w: id generation", ErrQualityReviewUnavailable)
	}
	record := QualityReviewCheckpoint{
		SchemaVersion: qualityReviewSchemaVersion,
		ID:            id, TenantID: tenantID, CreatedAt: now,
		Filter: qualityReviewFilterFromTrend(projection.Filter), Outcome: normalized.Outcome,
		Rationale: normalized.Rationale, GapIDs: append([]string(nil), normalized.GapIDs...),
		Snapshot: mapQualityReviewSnapshotV1(projection), IdempotencyKeyHash: keyHash, RequestFingerprint: fingerprint,
	}
	stored, created, err := s.store.AppendQualityReviewCheckpoint(record)
	if err != nil {
		if errors.Is(err, ErrQualityReviewIdempotencyConflict) {
			return QualityReviewCheckpointCreateResult{}, err
		}
		return QualityReviewCheckpointCreateResult{}, fmt.Errorf("%w: %w", ErrQualityReviewUnavailable, err)
	}
	return QualityReviewCheckpointCreateResult{Checkpoint: stored, Created: created}, nil
}

func (s QualityReviewCheckpointService) List(tenantID, spaceID string, limit int) (QualityReviewCheckpointHistory, error) {
	if s.store == nil || strings.TrimSpace(tenantID) == "" {
		return QualityReviewCheckpointHistory{}, ErrQualityReviewUnavailable
	}
	spaceID = strings.TrimSpace(spaceID)
	if spaceID != "" && validateKnowledgeID(spaceID) != nil {
		return QualityReviewCheckpointHistory{}, ErrQualityReviewInvalid
	}
	if limit < 1 || limit > 20 {
		return QualityReviewCheckpointHistory{}, ErrQualityReviewInvalid
	}
	records, excluded, err := s.store.ListQualityReviewCheckpoints(tenantID)
	if err != nil {
		return QualityReviewCheckpointHistory{}, fmt.Errorf("%w: %v", ErrQualityReviewUnavailable, err)
	}
	items := make([]QualityReviewCheckpoint, 0, len(records))
	for _, record := range records {
		if record.Filter.AllSpaces != (spaceID == "") || spaceID != "" && record.Filter.SpaceID != spaceID {
			continue
		}
		items = append(items, cloneQualityReviewCheckpoint(record))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	var comparison *QualityReviewComparison
	if len(items) > 0 && excluded == 0 {
		if previous, found := selectQualityReviewPredecessor(items[0], records); found {
			comparison = qualityReviewComparison(items[0], previous)
		}
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return QualityReviewCheckpointHistory{Checkpoints: items, ExcludedRecords: excluded, Comparison: comparison}, nil
}

func qualityReviewComparison(current, previous QualityReviewCheckpoint) *QualityReviewComparison {
	difference := func(metric string, currentValue, previousValue int) QualityReviewMetricDifference {
		return QualityReviewMetricDifference{Metric: metric, Status: "observed", Current: float64(currentValue), Previous: float64(previousValue), Delta: float64(currentValue - previousValue), Unit: "count"}
	}
	differences := []QualityReviewMetricDifference{
		difference("verification_attempt_count", current.Snapshot.Verification.AttemptCount, previous.Snapshot.Verification.AttemptCount),
		difference("passed_gap_count", current.Snapshot.Verification.PassedGapCount, previous.Snapshot.Verification.PassedGapCount),
		difference("suspected_recurrence_count", current.Snapshot.Recurrence.SuspectedCount, previous.Snapshot.Recurrence.SuspectedCount),
		difference("confirmed_recurrence_count", current.Snapshot.Recurrence.ConfirmedCount, previous.Snapshot.Recurrence.ConfirmedCount),
		difference("promoted_eval_failed_count", current.Snapshot.PromotedEval.FailedCount, previous.Snapshot.PromotedEval.FailedCount),
	}
	appendRate := func(metric string, currentMetric, previousMetric QualityReviewMetricSnapshotV1) {
		if currentMetric.Status != QualityTrendObserved || previousMetric.Status != QualityTrendObserved || currentMetric.Value == nil || previousMetric.Value == nil {
			differences = append(differences, QualityReviewMetricDifference{Metric: metric, Status: "not_comparable"})
			return
		}
		differences = append(differences, QualityReviewMetricDifference{Metric: metric, Status: "observed", Current: *currentMetric.Value, Previous: *previousMetric.Value, Delta: (*currentMetric.Value - *previousMetric.Value) * 100, Unit: "percentage_points"})
	}
	appendRate("recurrence_rate", current.Snapshot.Recurrence.Rate, previous.Snapshot.Recurrence.Rate)
	appendRate("promoted_eval_rate", current.Snapshot.PromotedEval.Rate, previous.Snapshot.PromotedEval.Rate)
	if current.Snapshot.Verification.TimeToVerify.Status == QualityTrendObserved && previous.Snapshot.Verification.TimeToVerify.Status == QualityTrendObserved && current.Snapshot.Verification.TimeToVerify.MedianMS != nil && previous.Snapshot.Verification.TimeToVerify.MedianMS != nil {
		differences = append(differences, QualityReviewMetricDifference{Metric: "time_to_verify_median", Status: "observed", Current: float64(*current.Snapshot.Verification.TimeToVerify.MedianMS), Previous: float64(*previous.Snapshot.Verification.TimeToVerify.MedianMS), Delta: float64(*current.Snapshot.Verification.TimeToVerify.MedianMS - *previous.Snapshot.Verification.TimeToVerify.MedianMS), Unit: "milliseconds"})
	} else {
		differences = append(differences, QualityReviewMetricDifference{Metric: "time_to_verify_median", Status: "not_comparable"})
	}
	return &QualityReviewComparison{CurrentID: current.ID, PreviousID: previous.ID, Current: current.Filter, Previous: previous.Filter, Differences: differences}
}

func (s QualityReviewCheckpointService) validateSelectedGaps(tenantID string, request QualityReviewCheckpointRequest, projection QualityTrendProjection) error {
	candidates := qualityReviewCandidateGaps(projection)
	byID := make(map[string]QualityReviewCandidateGap, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.GapID] = candidate
	}
	for _, gapID := range request.GapIDs {
		candidate, ok := byID[gapID]
		if !ok {
			return ErrQualityReviewGapNotEligible
		}
		gap, err := s.gaps.Get(tenantID, gapID)
		if err != nil || gap.TenantID != tenantID || gap.SpaceID != candidate.SpaceID {
			return ErrQualityReviewGapNotEligible
		}
		if !projection.Filter.AllSpaces && gap.SpaceID != projection.Filter.SpaceID {
			return ErrQualityReviewGapNotEligible
		}
	}
	return nil
}

func normalizeQualityReviewCheckpointRequest(request QualityReviewCheckpointRequest, now time.Time, location *time.Location) (QualityReviewCheckpointRequest, error) {
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	request.From = strings.TrimSpace(request.From)
	request.To = strings.TrimSpace(request.To)
	request.SpaceID = strings.TrimSpace(request.SpaceID)
	request.Rationale = strings.TrimSpace(request.Rationale)
	if len(request.IdempotencyKey) < qualityReviewKeyMinLength || len(request.IdempotencyKey) > qualityReviewKeyMaxLength || validateKnowledgeID(request.IdempotencyKey) != nil {
		return QualityReviewCheckpointRequest{}, ErrQualityReviewInvalid
	}
	if _, err := ParseQualityTrendFilter(request.From, request.To, request.SpaceID, now, location); err != nil {
		return QualityReviewCheckpointRequest{}, fmt.Errorf("%w: %v", ErrQualityReviewInvalid, err)
	}
	if request.Outcome != QualityReviewObserve && request.Outcome != QualityReviewRepairRequired && request.Outcome != QualityReviewRiskAccepted {
		return QualityReviewCheckpointRequest{}, ErrQualityReviewInvalid
	}
	if utf8.RuneCountInString(request.Rationale) > qualityReviewRationaleMax {
		return QualityReviewCheckpointRequest{}, ErrQualityReviewInvalid
	}
	if len(request.GapIDs) > qualityReviewCandidateLimit {
		return QualityReviewCheckpointRequest{}, ErrQualityReviewInvalid
	}
	seen := make(map[string]struct{}, len(request.GapIDs))
	for index, gapID := range request.GapIDs {
		gapID = strings.TrimSpace(gapID)
		if validateKnowledgeID(gapID) != nil {
			return QualityReviewCheckpointRequest{}, ErrQualityReviewInvalid
		}
		if _, ok := seen[gapID]; ok {
			return QualityReviewCheckpointRequest{}, ErrQualityReviewInvalid
		}
		seen[gapID] = struct{}{}
		request.GapIDs[index] = gapID
	}
	if request.Outcome == QualityReviewRepairRequired || request.Outcome == QualityReviewRiskAccepted {
		if request.Rationale == "" || len(request.GapIDs) == 0 {
			return QualityReviewCheckpointRequest{}, ErrQualityReviewInvalid
		}
	}
	sort.Strings(request.GapIDs)
	return request, nil
}

func qualityReviewFilterFromTrend(filter QualityTrendFilter) QualityReviewFilter {
	return QualityReviewFilter{
		From: filter.From, To: filter.To, SpaceID: filter.SpaceID, AllSpaces: filter.AllSpaces,
		Timezone: filter.Timezone, WindowDays: filter.WindowDays, Start: filter.Start, End: filter.End,
	}
}

type qualityReviewFingerprintInput struct {
	TenantID  string               `json:"tenant_id"`
	From      string               `json:"from"`
	To        string               `json:"to"`
	SpaceID   string               `json:"space_id"`
	Outcome   QualityReviewOutcome `json:"outcome"`
	Rationale string               `json:"rationale"`
	GapIDs    []string             `json:"gap_ids"`
}

func qualityReviewRequestFingerprint(tenantID string, request QualityReviewCheckpointRequest) (string, error) {
	encoded, err := json.Marshal(qualityReviewFingerprintInput{
		TenantID: tenantID, From: request.From, To: request.To, SpaceID: request.SpaceID,
		Outcome: request.Outcome, Rationale: request.Rationale, GapIDs: request.GapIDs,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func qualityReviewIdempotencyKeyHash(tenantID, key string) string {
	sum := sha256.Sum256([]byte(tenantID + "\x00" + key))
	return hex.EncodeToString(sum[:])
}

func newQualityReviewCheckpointID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "quality-review-" + hex.EncodeToString(bytes), nil
}

type InMemoryQualityReviewCheckpointStore struct {
	mu      sync.Mutex
	records []QualityReviewCheckpoint
}

func NewInMemoryQualityReviewCheckpointStore() *InMemoryQualityReviewCheckpointStore {
	return &InMemoryQualityReviewCheckpointStore{}
}

func (s *InMemoryQualityReviewCheckpointStore) FindQualityReviewCheckpointByIdempotency(tenantID, keyHash string) (QualityReviewCheckpoint, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, record := range s.records {
		if record.TenantID == tenantID && record.IdempotencyKeyHash == keyHash {
			return cloneQualityReviewCheckpoint(record), true, nil
		}
	}
	return QualityReviewCheckpoint{}, false, nil
}

func (s *InMemoryQualityReviewCheckpointStore) AppendQualityReviewCheckpoint(record QualityReviewCheckpoint) (QualityReviewCheckpoint, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.records {
		if existing.TenantID != record.TenantID || existing.IdempotencyKeyHash != record.IdempotencyKeyHash {
			continue
		}
		if existing.RequestFingerprint != record.RequestFingerprint {
			return QualityReviewCheckpoint{}, false, ErrQualityReviewIdempotencyConflict
		}
		return cloneQualityReviewCheckpoint(existing), false, nil
	}
	s.records = append(s.records, cloneQualityReviewCheckpoint(record))
	return cloneQualityReviewCheckpoint(record), true, nil
}

func (s *InMemoryQualityReviewCheckpointStore) ListQualityReviewCheckpoints(tenantID string) ([]QualityReviewCheckpoint, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]QualityReviewCheckpoint, 0)
	for _, record := range s.records {
		if record.TenantID == tenantID {
			items = append(items, cloneQualityReviewCheckpoint(record))
		}
	}
	return items, 0, nil
}

func cloneQualityReviewCheckpoint(record QualityReviewCheckpoint) QualityReviewCheckpoint {
	cloned := record
	cloned.GapIDs = append([]string(nil), record.GapIDs...)
	cloned.Snapshot = cloneQualityReviewSnapshotV1(record.Snapshot)
	return cloned
}

func cloneQualityReviewSnapshotV1(snapshot QualityReviewSnapshotV1) QualityReviewSnapshotV1 {
	cloned := snapshot
	cloned.Verification.TimeToVerify = cloneQualityReviewMetricSnapshotV1(snapshot.Verification.TimeToVerify)
	cloned.Verification.CurrentStale = cloneQualityReviewMetricSnapshotV1(snapshot.Verification.CurrentStale)
	cloned.Recurrence.Rate = cloneQualityReviewMetricSnapshotV1(snapshot.Recurrence.Rate)
	cloned.PromotedEval.Rate = cloneQualityReviewMetricSnapshotV1(snapshot.PromotedEval.Rate)
	return cloned
}

func cloneQualityReviewMetricSnapshotV1(metric QualityReviewMetricSnapshotV1) QualityReviewMetricSnapshotV1 {
	cloned := metric
	if metric.Value != nil {
		value := *metric.Value
		cloned.Value = &value
	}
	if metric.MedianMS != nil {
		median := *metric.MedianMS
		cloned.MedianMS = &median
	}
	return cloned
}

type QualityReviewCandidateGap struct {
	GapID   string `json:"gap_id"`
	SpaceID string `json:"space_id"`
}

func mapQualityReviewSnapshotV1(projection QualityTrendProjection) QualityReviewSnapshotV1 {
	return QualityReviewSnapshotV1{
		ProjectedAt: projection.ProjectedAt,
		Verification: QualityReviewVerificationSnapshotV1{
			AttemptCount:   projection.Verification.AttemptCount,
			PassedGapCount: projection.Verification.PassedGapCount,
			TimeToVerify:   mapQualityReviewMetricSnapshotV1(projection.Verification.TimeToVerify),
			CurrentStale:   mapQualityReviewMetricSnapshotV1(projection.Verification.Stale),
		},
		Recurrence: QualityReviewRecurrenceSnapshotV1{
			SuspectedCount: projection.Recurrence.SuspectedCount,
			ConfirmedCount: projection.Recurrence.ConfirmedCount,
			DismissedCount: projection.Recurrence.DismissedCount,
			Rate:           mapQualityReviewMetricSnapshotV1(projection.Recurrence.Rate),
		},
		PromotedEval: QualityReviewPromotedEvalSnapshotV1{
			PassedCount:      projection.PromotedEval.PassedCount,
			FailedCount:      projection.PromotedEval.FailedCount,
			UnavailableCount: projection.PromotedEval.UnavailableCount,
			Rate:             mapQualityReviewMetricSnapshotV1(projection.PromotedEval.Rate),
		},
	}
}

func mapQualityReviewMetricSnapshotV1(metric QualityTrendMetric) QualityReviewMetricSnapshotV1 {
	result := QualityReviewMetricSnapshotV1{
		Status: metric.Status, Count: metric.Count, Denominator: metric.Denominator, ExcludedCount: metric.ExcludedCount,
	}
	if metric.Value != nil {
		value := *metric.Value
		result.Value = &value
	}
	if metric.MedianMS != nil {
		median := *metric.MedianMS
		result.MedianMS = &median
	}
	return result
}

func qualityReviewCandidateGaps(projection QualityTrendProjection) []QualityReviewCandidateGap {
	items := make([]QualityReviewCandidateGap, 0, qualityReviewCandidateLimit)
	seen := make(map[string]struct{}, qualityReviewCandidateLimit)
	appendCandidate := func(gapID, spaceID string) {
		if gapID == "" || len(items) >= qualityReviewCandidateLimit {
			return
		}
		if _, ok := seen[gapID]; ok {
			return
		}
		seen[gapID] = struct{}{}
		items = append(items, QualityReviewCandidateGap{GapID: gapID, SpaceID: spaceID})
	}
	for _, question := range projection.RepeatedFailures.Questions {
		appendCandidate(question.GapID, question.SpaceID)
	}
	for _, failure := range projection.RepeatedFailures.PromotedCases {
		appendCandidate(failure.GapID, failure.SpaceID)
	}
	return items
}

func selectQualityReviewPredecessor(current QualityReviewCheckpoint, records []QualityReviewCheckpoint) (QualityReviewCheckpoint, bool) {
	var selected QualityReviewCheckpoint
	found := false
	for _, candidate := range records {
		if !qualityReviewCompatiblePredecessor(current, candidate) {
			continue
		}
		if !found || candidate.Filter.End.After(selected.Filter.End) || candidate.Filter.End.Equal(selected.Filter.End) && (candidate.CreatedAt.After(selected.CreatedAt) || candidate.CreatedAt.Equal(selected.CreatedAt) && candidate.ID > selected.ID) {
			selected = candidate
			found = true
		}
	}
	return selected, found
}

func qualityReviewCompatiblePredecessor(current, candidate QualityReviewCheckpoint) bool {
	return current.TenantID == candidate.TenantID &&
		current.Filter.AllSpaces == candidate.Filter.AllSpaces &&
		current.Filter.SpaceID == candidate.Filter.SpaceID &&
		current.Filter.Timezone == candidate.Filter.Timezone &&
		current.Filter.WindowDays == candidate.Filter.WindowDays &&
		candidate.Filter.End.Before(current.Filter.End)
}

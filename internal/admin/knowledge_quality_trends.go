package admin

import (
	"errors"
	"sort"
	"strings"
	"time"
)

const (
	QualityTrendObserved           = "observed"
	QualityTrendInsufficientSample = "insufficient_sample"
	QualityTrendNoObservations     = "no_observations"
	QualityTrendUnavailable        = "unavailable"
)

const (
	qualityTrendDefaultDays = 30
	qualityTrendMaxDays     = 90
	qualityTrendMinSample   = 3
	qualityTrendSourceLimit = 10000
	qualityTrendTraceLimit  = 20
)

type QualityTrendRequest struct {
	From    string
	To      string
	SpaceID string
}

type QualityTrendFilter struct {
	From       string    `json:"from"`
	To         string    `json:"to"`
	SpaceID    string    `json:"space_id,omitempty"`
	AllSpaces  bool      `json:"all_spaces"`
	Timezone   string    `json:"timezone"`
	WindowDays int       `json:"window_days"`
	Start      time.Time `json:"start_at"`
	End        time.Time `json:"end_exclusive_at"`
}

type QualityTrendMetric struct {
	Status        string   `json:"status"`
	Count         int      `json:"count"`
	Denominator   int      `json:"denominator,omitempty"`
	Value         *float64 `json:"value,omitempty"`
	MedianMS      *int64   `json:"median_ms,omitempty"`
	ExcludedCount int      `json:"excluded_count,omitempty"`
}

type QualityTrendVerification struct {
	AttemptCount   int                `json:"attempt_count"`
	PassedGapCount int                `json:"passed_gap_count"`
	TimeToVerify   QualityTrendMetric `json:"time_to_verify"`
	Stale          QualityTrendMetric `json:"stale"`
}

type QualityTrendRecurrence struct {
	SuspectedCount int                `json:"suspected_count"`
	ConfirmedCount int                `json:"confirmed_count"`
	DismissedCount int                `json:"dismissed_count"`
	Rate           QualityTrendMetric `json:"rate"`
}

type QualityTrendEval struct {
	PassedCount      int                      `json:"passed_count"`
	FailedCount      int                      `json:"failed_count"`
	UnavailableCount int                      `json:"unavailable_count"`
	Rate             QualityTrendMetric       `json:"rate"`
	Buckets          []QualityTrendEvalBucket `json:"buckets,omitempty"`
}

type QualityTrendEvalBucket struct {
	StartDate        string `json:"start_date"`
	PassedCount      int    `json:"passed_count"`
	FailedCount      int    `json:"failed_count"`
	UnavailableCount int    `json:"unavailable_count"`
	TotalCount       int    `json:"total_count"`
}

type QualityTrendQuestionFailure struct {
	GapID           string `json:"gap_id"`
	SpaceID         string `json:"space_id"`
	QuestionSummary string `json:"question_summary"`
	OccurrenceCount int    `json:"occurrence_count"`
	ConfirmedCount  int    `json:"confirmed_count"`
}

type QualityTrendSpaceFailure struct {
	SpaceID         string `json:"space_id"`
	OccurrenceCount int    `json:"occurrence_count"`
	ConfirmedCount  int    `json:"confirmed_count"`
}

type QualityTrendPromotedFailure struct {
	CaseID      string    `json:"case_id"`
	GapID       string    `json:"gap_id"`
	SpaceID     string    `json:"space_id"`
	FailedCount int       `json:"failed_count"`
	LatestAt    time.Time `json:"latest_at"`
}

type QualityTrendRepeatedFailures struct {
	Questions     []QualityTrendQuestionFailure `json:"questions,omitempty"`
	Spaces        []QualityTrendSpaceFailure    `json:"spaces,omitempty"`
	PromotedCases []QualityTrendPromotedFailure `json:"promoted_cases,omitempty"`
}

type QualityTrendTrace struct {
	GapIDs        []string `json:"gap_ids,omitempty"`
	AttemptIDs    []string `json:"verification_attempt_ids,omitempty"`
	RecurrenceIDs []string `json:"recurrence_ids,omitempty"`
	EvalIDs       []string `json:"eval_observation_ids,omitempty"`
	Truncated     bool     `json:"truncated"`
}

type QualityTrendProjection struct {
	ProjectedAt      time.Time                    `json:"projected_at"`
	Filter           QualityTrendFilter           `json:"filter"`
	Verification     QualityTrendVerification     `json:"verification"`
	Recurrence       QualityTrendRecurrence       `json:"recurrence"`
	PromotedEval     QualityTrendEval             `json:"promoted_eval"`
	RepeatedFailures QualityTrendRepeatedFailures `json:"repeated_failures"`
	Trace            QualityTrendTrace            `json:"trace"`
}

type QualityTrendDependencies struct {
	Gaps         KnowledgeGapService
	Knowledge    KnowledgeService
	Verification RepairVerificationService
	Recurrences  RepairRecurrenceStore
	Observations RepairEvalObservationStore
	Now          func() time.Time
	Location     *time.Location
}

type QualityTrendService struct {
	gaps         KnowledgeGapService
	knowledge    KnowledgeService
	verification RepairVerificationService
	recurrences  RepairRecurrenceStore
	observations RepairEvalObservationStore
	now          func() time.Time
	location     *time.Location
}

func NewQualityTrendService(dependencies QualityTrendDependencies) QualityTrendService {
	now := dependencies.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	location := dependencies.Location
	if location == nil {
		location = time.Local
	}
	return QualityTrendService{
		gaps: dependencies.Gaps, knowledge: dependencies.Knowledge, verification: dependencies.Verification,
		recurrences: dependencies.Recurrences, observations: dependencies.Observations,
		now: now, location: location,
	}
}

func ParseQualityTrendFilter(from, to, spaceID string, now time.Time, location *time.Location) (QualityTrendFilter, error) {
	if location == nil {
		location = time.Local
	}
	if now.IsZero() {
		now = time.Now()
	}
	localNow := now.In(location)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	if from == "" && to == "" {
		end := today.AddDate(0, 0, 1)
		start := end.AddDate(0, 0, -qualityTrendDefaultDays)
		return qualityTrendFilterWithContext(start, end, spaceID, location), nil
	}
	if from == "" || to == "" {
		return QualityTrendFilter{}, errors.New("from and to must be provided together")
	}
	startDate, err := time.ParseInLocation("2006-01-02", from, location)
	if err != nil {
		return QualityTrendFilter{}, errors.New("invalid from date")
	}
	endDate, err := time.ParseInLocation("2006-01-02", to, location)
	if err != nil {
		return QualityTrendFilter{}, errors.New("invalid to date")
	}
	end := endDate.AddDate(0, 0, 1)
	days := qualityTrendCalendarDays(startDate, end, location)
	if days < 1 || days > qualityTrendMaxDays {
		return QualityTrendFilter{}, errors.New("quality trend date range must be between 1 and 90 days")
	}
	return qualityTrendFilterWithContext(startDate, end, spaceID, location), nil
}

func qualityTrendFilterWithContext(start, end time.Time, spaceID string, location *time.Location) QualityTrendFilter {
	return QualityTrendFilter{
		From:       start.In(location).Format("2006-01-02"),
		To:         end.AddDate(0, 0, -1).In(location).Format("2006-01-02"),
		SpaceID:    normalizeDocumentSpaceID(spaceID),
		AllSpaces:  strings.TrimSpace(spaceID) == "",
		Timezone:   location.String(),
		WindowDays: qualityTrendCalendarDays(start, end, location),
		Start:      start,
		End:        end,
	}
}

func (s QualityTrendService) Project(tenantID string, request QualityTrendRequest) (QualityTrendProjection, error) {
	if s.gaps.store == nil || s.knowledge.store == nil || s.verification.store == nil || s.verification.knowledge.store == nil || s.recurrences == nil {
		return QualityTrendProjection{}, errors.New("quality trend service unavailable")
	}
	projectedAt := s.now()
	filter, err := ParseQualityTrendFilter(request.From, request.To, request.SpaceID, projectedAt, s.location)
	if err != nil {
		return QualityTrendProjection{}, err
	}
	gaps, err := s.gaps.ListAll(tenantID)
	if err != nil {
		return QualityTrendProjection{}, err
	}
	gapByID := make(map[string]KnowledgeGap, len(gaps))
	for _, gap := range gaps {
		if !filter.AllSpaces && gap.SpaceID != filter.SpaceID {
			continue
		}
		gapByID[gap.ID] = gap
	}
	attempts, err := s.verification.ListAll(tenantID)
	if err != nil {
		return QualityTrendProjection{}, err
	}
	projection := QualityTrendProjection{ProjectedAt: projectedAt, Filter: filter}
	passedByGap := make(map[string]RepairVerificationAttempt)
	for _, attempt := range attempts {
		if _, ok := gapByID[attempt.GapID]; !ok || !filter.AllSpaces && attempt.SpaceID != filter.SpaceID {
			continue
		}
		if !inQualityTrendWindow(attempt.CompletedAt, filter) {
			continue
		}
		projection.Verification.AttemptCount++
		appendQualityTrendTrace(&projection.Trace.AttemptIDs, attempt.ID, &projection.Trace.Truncated)
		if attempt.Result != RepairVerificationPassed {
			continue
		}
		if previous, ok := passedByGap[attempt.GapID]; !ok || attempt.CompletedAt.Before(previous.CompletedAt) {
			passedByGap[attempt.GapID] = attempt
		}
	}
	projection.Verification.PassedGapCount = len(passedByGap)
	durations := make([]int64, 0, len(passedByGap))
	for gapID, attempt := range passedByGap {
		gap := gapByID[gapID]
		if gap.CreatedAt.IsZero() || gap.CreatedAt.After(attempt.CompletedAt) {
			projection.Verification.TimeToVerify.ExcludedCount++
			continue
		}
		durations = append(durations, attempt.CompletedAt.Sub(gap.CreatedAt).Milliseconds())
		appendQualityTrendTrace(&projection.Trace.GapIDs, gap.ID, &projection.Trace.Truncated)
	}
	projection.Verification.TimeToVerify = qualityTrendDurationMetric(durations, projection.Verification.TimeToVerify.ExcludedCount)
	staleCount := 0
	resolvedCount := 0
	for _, gap := range gapByID {
		if gap.Status != KnowledgeGapResolved {
			continue
		}
		resolvedCount++
		current, projectionErr := s.verification.CurrentProjection(tenantID, gap)
		if projectionErr != nil {
			return QualityTrendProjection{}, projectionErr
		}
		if current.State == RepairVerificationStale {
			staleCount++
		}
	}
	if resolvedCount == 0 {
		projection.Verification.Stale.Status = QualityTrendNoObservations
	} else {
		projection.Verification.Stale.Status = QualityTrendObserved
	}
	projection.Verification.Stale.Count = staleCount
	recurrences, err := s.recurrences.ListRepairRecurrences(tenantID, "", "", qualityTrendSourceLimit)
	if err != nil {
		return QualityTrendProjection{}, err
	}
	recurrenceGaps := make(map[string]struct{})
	recurrenceByGap := make(map[string]QualityTrendQuestionFailure)
	recurrenceBySpace := make(map[string]QualityTrendSpaceFailure)
	for _, recurrence := range recurrences {
		if _, ok := gapByID[recurrence.GapID]; !ok || !inQualityTrendWindow(recurrence.CreatedAt, filter) {
			continue
		}
		recurrenceGaps[recurrence.GapID] = struct{}{}
		appendQualityTrendTrace(&projection.Trace.RecurrenceIDs, recurrence.ID, &projection.Trace.Truncated)
		switch recurrence.Status {
		case RepairRecurrenceSuspected:
			projection.Recurrence.SuspectedCount++
		case RepairRecurrenceConfirmed:
			projection.Recurrence.ConfirmedCount++
		case RepairRecurrenceDismissed:
			projection.Recurrence.DismissedCount++
		}
		question := recurrenceByGap[recurrence.GapID]
		gap := gapByID[recurrence.GapID]
		question.GapID = gap.ID
		question.SpaceID = gap.SpaceID
		question.QuestionSummary = truncateQualityTrendQuestion(gap.Question)
		question.OccurrenceCount += recurrence.OccurrenceCount
		if recurrence.Status == RepairRecurrenceConfirmed {
			question.ConfirmedCount++
		}
		recurrenceByGap[recurrence.GapID] = question
		space := recurrenceBySpace[recurrence.SpaceID]
		space.SpaceID = recurrence.SpaceID
		space.OccurrenceCount += recurrence.OccurrenceCount
		if recurrence.Status == RepairRecurrenceConfirmed {
			space.ConfirmedCount++
		}
		recurrenceBySpace[recurrence.SpaceID] = space
	}
	projection.Recurrence.Rate = qualityTrendRateMetric(len(recurrenceGaps), len(passedByGap))
	projection.RepeatedFailures.Questions = rankQualityTrendQuestions(recurrenceByGap)
	projection.RepeatedFailures.Spaces = rankQualityTrendSpaces(recurrenceBySpace)
	if s.observations == nil {
		projection.PromotedEval.Rate.Status = QualityTrendUnavailable
		return projection, nil
	}
	observations, _, err := s.observations.ListRepairEvalObservations(tenantID, qualityTrendSourceLimit)
	if err != nil {
		return QualityTrendProjection{}, err
	}
	totalEvaluations := 0
	promotedFailures := make(map[string]QualityTrendPromotedFailure)
	selectedObservations := make([]RepairEvalObservation, 0)
	for _, observation := range observations {
		if _, ok := gapByID[observation.GapID]; !ok || !filter.AllSpaces && observation.SpaceID != filter.SpaceID || !inQualityTrendWindow(observation.ObservedAt, filter) {
			continue
		}
		totalEvaluations++
		selectedObservations = append(selectedObservations, observation)
		appendQualityTrendTrace(&projection.Trace.EvalIDs, observation.ID, &projection.Trace.Truncated)
		switch observation.Status {
		case RepairEvalObservationPassed:
			projection.PromotedEval.PassedCount++
		case RepairEvalObservationFailed:
			projection.PromotedEval.FailedCount++
		case RepairEvalObservationUnavailable:
			projection.PromotedEval.UnavailableCount++
		}
		failure := promotedFailures[observation.CaseID]
		failure.CaseID = observation.CaseID
		failure.GapID = observation.GapID
		failure.SpaceID = observation.SpaceID
		if observation.Status == RepairEvalObservationFailed {
			failure.FailedCount++
		}
		if observation.ObservedAt.After(failure.LatestAt) {
			failure.LatestAt = observation.ObservedAt
		}
		promotedFailures[observation.CaseID] = failure
	}
	projection.PromotedEval.Rate = qualityTrendRateMetric(projection.PromotedEval.PassedCount, totalEvaluations)
	projection.RepeatedFailures.PromotedCases = rankQualityTrendPromotedFailures(promotedFailures)
	projection.PromotedEval.Buckets = qualityTrendEvalBuckets(selectedObservations, filter, s.location)
	return projection, nil
}

func inQualityTrendWindow(value time.Time, filter QualityTrendFilter) bool {
	return !value.Before(filter.Start) && value.Before(filter.End)
}

func qualityTrendDurationMetric(values []int64, excluded int) QualityTrendMetric {
	metric := QualityTrendMetric{Count: len(values), ExcludedCount: excluded}
	if len(values) == 0 {
		metric.Status = QualityTrendNoObservations
		return metric
	}
	if len(values) < qualityTrendMinSample {
		metric.Status = QualityTrendInsufficientSample
		return metric
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	median := values[len(values)/2]
	if len(values)%2 == 0 {
		median = (values[len(values)/2-1] + values[len(values)/2]) / 2
	}
	metric.Status = QualityTrendObserved
	metric.MedianMS = &median
	return metric
}

func qualityTrendRateMetric(numerator, denominator int) QualityTrendMetric {
	metric := QualityTrendMetric{Count: numerator, Denominator: denominator}
	if denominator == 0 {
		metric.Status = QualityTrendNoObservations
		return metric
	}
	if denominator < qualityTrendMinSample {
		metric.Status = QualityTrendInsufficientSample
		return metric
	}
	value := float64(numerator) / float64(denominator)
	metric.Status = QualityTrendObserved
	metric.Value = &value
	return metric
}

func appendQualityTrendTrace(values *[]string, value string, truncated *bool) {
	if strings.TrimSpace(value) == "" {
		return
	}
	for _, existing := range *values {
		if existing == value {
			return
		}
	}
	if len(*values) >= qualityTrendTraceLimit {
		*truncated = true
		return
	}
	*values = append(*values, value)
}

func truncateQualityTrendQuestion(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= 160 {
		return value
	}
	return string(runes[:160])
}

func rankQualityTrendQuestions(values map[string]QualityTrendQuestionFailure) []QualityTrendQuestionFailure {
	items := make([]QualityTrendQuestionFailure, 0, len(values))
	for _, value := range values {
		items = append(items, value)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].OccurrenceCount != items[j].OccurrenceCount {
			return items[i].OccurrenceCount > items[j].OccurrenceCount
		}
		if items[i].ConfirmedCount != items[j].ConfirmedCount {
			return items[i].ConfirmedCount > items[j].ConfirmedCount
		}
		return items[i].GapID < items[j].GapID
	})
	if len(items) > 5 {
		items = items[:5]
	}
	return items
}

func rankQualityTrendSpaces(values map[string]QualityTrendSpaceFailure) []QualityTrendSpaceFailure {
	items := make([]QualityTrendSpaceFailure, 0, len(values))
	for _, value := range values {
		items = append(items, value)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].OccurrenceCount != items[j].OccurrenceCount {
			return items[i].OccurrenceCount > items[j].OccurrenceCount
		}
		return items[i].SpaceID < items[j].SpaceID
	})
	if len(items) > 5 {
		items = items[:5]
	}
	return items
}

func rankQualityTrendPromotedFailures(values map[string]QualityTrendPromotedFailure) []QualityTrendPromotedFailure {
	items := make([]QualityTrendPromotedFailure, 0, len(values))
	for _, value := range values {
		if value.FailedCount > 0 {
			items = append(items, value)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].FailedCount != items[j].FailedCount {
			return items[i].FailedCount > items[j].FailedCount
		}
		if !items[i].LatestAt.Equal(items[j].LatestAt) {
			return items[i].LatestAt.After(items[j].LatestAt)
		}
		return items[i].CaseID < items[j].CaseID
	})
	if len(items) > 5 {
		items = items[:5]
	}
	return items
}

func qualityTrendEvalBuckets(observations []RepairEvalObservation, filter QualityTrendFilter, location *time.Location) []QualityTrendEvalBucket {
	days := qualityTrendCalendarDays(filter.Start, filter.End, location)
	bucketCount := (days + 6) / 7
	if bucketCount > 13 {
		bucketCount = 13
	}
	buckets := make([]QualityTrendEvalBucket, bucketCount)
	for index := range buckets {
		buckets[index].StartDate = filter.Start.AddDate(0, 0, index*7).In(location).Format("2006-01-02")
	}
	for _, observation := range observations {
		observationLocal := observation.ObservedAt.In(location)
		startLocal := filter.Start.In(location)
		observationCivil := time.Date(observationLocal.Year(), observationLocal.Month(), observationLocal.Day(), 0, 0, 0, 0, time.UTC)
		startCivil := time.Date(startLocal.Year(), startLocal.Month(), startLocal.Day(), 0, 0, 0, 0, time.UTC)
		index := int(observationCivil.Sub(startCivil).Hours() / (24 * 7))
		if index < 0 || index >= len(buckets) {
			continue
		}
		buckets[index].TotalCount++
		switch observation.Status {
		case RepairEvalObservationPassed:
			buckets[index].PassedCount++
		case RepairEvalObservationFailed:
			buckets[index].FailedCount++
		case RepairEvalObservationUnavailable:
			buckets[index].UnavailableCount++
		}
	}
	return buckets
}

func qualityTrendCalendarDays(start, end time.Time, location *time.Location) int {
	startLocal := start.In(location)
	endLocal := end.In(location)
	startCivil := time.Date(startLocal.Year(), startLocal.Month(), startLocal.Day(), 0, 0, 0, 0, time.UTC)
	endCivil := time.Date(endLocal.Year(), endLocal.Month(), endLocal.Day(), 0, 0, 0, 0, time.UTC)
	return int(endCivil.Sub(startCivil).Hours() / 24)
}

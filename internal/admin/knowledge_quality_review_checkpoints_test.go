package admin

import (
	"errors"
	"testing"
	"time"
)

func TestMapQualityReviewSnapshotV1CopiesOnlyAllowedFields(t *testing.T) {
	rate := 0.5
	median := int64(42)
	projection := QualityTrendProjection{
		ProjectedAt: time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC),
		Verification: QualityTrendVerification{
			AttemptCount:   4,
			PassedGapCount: 3,
			TimeToVerify:   QualityTrendMetric{Status: QualityTrendObserved, Count: 3, MedianMS: &median},
			Stale:          QualityTrendMetric{Status: QualityTrendObserved, Count: 1},
		},
		Recurrence:       QualityTrendRecurrence{SuspectedCount: 2, ConfirmedCount: 1, DismissedCount: 1, Rate: QualityTrendMetric{Status: QualityTrendObserved, Count: 2, Denominator: 4, Value: &rate}},
		PromotedEval:     QualityTrendEval{PassedCount: 2, FailedCount: 1, UnavailableCount: 1, Rate: QualityTrendMetric{Status: QualityTrendObserved, Count: 2, Denominator: 4, Value: &rate}, Buckets: []QualityTrendEvalBucket{{StartDate: "2026-07-13", TotalCount: 4}}},
		RepeatedFailures: QualityTrendRepeatedFailures{Questions: []QualityTrendQuestionFailure{{GapID: "gap-1", QuestionSummary: "sensitive"}}},
		Trace:            QualityTrendTrace{GapIDs: []string{"gap-1"}},
	}

	snapshot := mapQualityReviewSnapshotV1(projection)
	if !snapshot.ProjectedAt.Equal(projection.ProjectedAt) || snapshot.Verification.AttemptCount != 4 || snapshot.Recurrence.Rate.Denominator != 4 || snapshot.PromotedEval.Rate.Value == nil || *snapshot.PromotedEval.Rate.Value != rate {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestQualityReviewCandidateGapsUsesProjectionEvidenceInStableOrder(t *testing.T) {
	projection := QualityTrendProjection{RepeatedFailures: QualityTrendRepeatedFailures{
		Questions:     []QualityTrendQuestionFailure{{GapID: "gap-1", SpaceID: "ops"}, {GapID: "gap-2", SpaceID: "ops"}},
		PromotedCases: []QualityTrendPromotedFailure{{GapID: "gap-2", SpaceID: "ops"}, {GapID: "gap-3", SpaceID: "support"}},
	}}

	candidates := qualityReviewCandidateGaps(projection)
	if len(candidates) != 3 || candidates[0].GapID != "gap-1" || candidates[1].GapID != "gap-2" || candidates[2].GapID != "gap-3" {
		t.Fatalf("candidates = %#v", candidates)
	}
}

func TestSelectQualityReviewPredecessorUsesLatestEarlierCompatibleWindow(t *testing.T) {
	current := qualityReviewCheckpointForTest("current", "2026-07-10", "2026-07-12", time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC))
	older := qualityReviewCheckpointForTest("older", "2026-06-30", "2026-07-02", time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC))
	latest := qualityReviewCheckpointForTest("latest", "2026-07-05", "2026-07-07", time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC))
	exact := qualityReviewCheckpointForTest("exact", "2026-07-10", "2026-07-12", time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC))
	wrongSpace := qualityReviewCheckpointForTest("wrong-space", "2026-07-05", "2026-07-07", time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC))
	wrongSpace.Filter.SpaceID = "support"

	predecessor, ok := selectQualityReviewPredecessor(current, []QualityReviewCheckpoint{older, exact, wrongSpace, latest})
	if !ok || predecessor.ID != "latest" {
		t.Fatalf("predecessor = %#v, ok=%v", predecessor, ok)
	}
}

func TestQualityReviewComparisonWithholdsRateDifferenceWhenEitherWindowIsNotObserved(t *testing.T) {
	current := qualityReviewCheckpointForTest("current", "2026-07-10", "2026-07-12", time.Now().UTC())
	previous := qualityReviewCheckpointForTest("previous", "2026-07-01", "2026-07-03", time.Now().UTC())
	current.Snapshot.Recurrence.Rate.Status = QualityTrendObserved
	previous.Snapshot.Recurrence.Rate.Status = QualityTrendInsufficientSample
	comparison := qualityReviewComparison(current, previous)
	for _, difference := range comparison.Differences {
		if difference.Metric == "recurrence_rate" && difference.Status != "not_comparable" {
			t.Fatalf("recurrence difference = %#v", difference)
		}
	}
}

func TestCreateQualityReviewCheckpointReplaysBeforeProjection(t *testing.T) {
	projector := &qualityReviewTrendProjectorFake{projection: QualityTrendProjection{
		ProjectedAt: time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC),
		Filter:      QualityTrendFilter{From: "2026-07-10", To: "2026-07-12", AllSpaces: true, Timezone: "UTC", WindowDays: 3, Start: time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)},
	}}
	service := NewQualityReviewCheckpointService(QualityReviewCheckpointDependencies{
		Trends: projector,
		Gaps:   qualityReviewGapReaderFake{},
		Store:  NewInMemoryQualityReviewCheckpointStore(),
		Now:    func() time.Time { return time.Date(2026, 7, 13, 12, 1, 0, 0, time.UTC) },
		NewID:  func() (string, error) { return "checkpoint-1", nil },
	})
	request := QualityReviewCheckpointRequest{IdempotencyKey: "checkpoint-request-1", From: "2026-07-10", To: "2026-07-12", Outcome: QualityReviewObserve}

	created, err := service.Create("tenant-a", request)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}
	if !created.Created || created.Checkpoint.ID != "checkpoint-1" || projector.calls != 1 {
		t.Fatalf("created = %#v, calls=%d", created, projector.calls)
	}

	projector.err = errors.New("projection should not run on replay")
	replayed, err := service.Create("tenant-a", request)
	if err != nil {
		t.Fatalf("replay returned error: %v", err)
	}
	if replayed.Created || replayed.Checkpoint.ID != created.Checkpoint.ID || projector.calls != 1 {
		t.Fatalf("replayed = %#v, calls=%d", replayed, projector.calls)
	}
}

func TestCreateQualityReviewCheckpointRejectsGapOutsideProjectionCandidates(t *testing.T) {
	projector := &qualityReviewTrendProjectorFake{projection: QualityTrendProjection{
		ProjectedAt:      time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC),
		Filter:           QualityTrendFilter{From: "2026-07-10", To: "2026-07-12", AllSpaces: true, Timezone: "UTC", WindowDays: 3, Start: time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)},
		RepeatedFailures: QualityTrendRepeatedFailures{Questions: []QualityTrendQuestionFailure{{GapID: "gap-eligible", SpaceID: "ops"}}},
	}}
	service := NewQualityReviewCheckpointService(QualityReviewCheckpointDependencies{
		Trends: projector,
		Gaps:   qualityReviewGapReaderFake{"gap-other": {ID: "gap-other", TenantID: "tenant-a", SpaceID: "ops"}},
		Store:  NewInMemoryQualityReviewCheckpointStore(),
		Now:    func() time.Time { return time.Date(2026, 7, 13, 12, 1, 0, 0, time.UTC) },
		NewID:  func() (string, error) { return "checkpoint-1", nil },
	})
	_, err := service.Create("tenant-a", QualityReviewCheckpointRequest{
		IdempotencyKey: "checkpoint-request-2", From: "2026-07-10", To: "2026-07-12",
		Outcome: QualityReviewRepairRequired, Rationale: "repair the observed gap", GapIDs: []string{"gap-other"},
	})
	if !errors.Is(err, ErrQualityReviewGapNotEligible) {
		t.Fatalf("create error = %v, want gap eligibility error", err)
	}
}

type qualityReviewTrendProjectorFake struct {
	projection QualityTrendProjection
	err        error
	calls      int
}

func (f *qualityReviewTrendProjectorFake) Project(string, QualityTrendRequest) (QualityTrendProjection, error) {
	f.calls++
	if f.err != nil {
		return QualityTrendProjection{}, f.err
	}
	return f.projection, nil
}

type qualityReviewGapReaderFake map[string]KnowledgeGap

func (f qualityReviewGapReaderFake) Get(_ string, gapID string) (KnowledgeGap, error) {
	gap, ok := f[gapID]
	if !ok {
		return KnowledgeGap{}, errors.New("not found")
	}
	return gap, nil
}

func qualityReviewCheckpointForTest(id, from, to string, createdAt time.Time) QualityReviewCheckpoint {
	start, _ := time.Parse("2006-01-02", from)
	endDate, _ := time.Parse("2006-01-02", to)
	return QualityReviewCheckpoint{
		ID:        id,
		TenantID:  "tenant-a",
		CreatedAt: createdAt,
		Filter: QualityReviewFilter{
			From: from, To: to, SpaceID: "ops", Timezone: "UTC", WindowDays: 3,
			Start: start, End: endDate.AddDate(0, 0, 1),
		},
	}
}

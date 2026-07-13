package admin

import (
	"strings"
	"testing"
	"time"
)

func TestParseQualityTrendFilterUsesBoundedLocalDates(t *testing.T) {
	now := time.Date(2026, 7, 11, 15, 30, 0, 0, time.UTC)
	location := time.FixedZone("CST", 8*60*60)
	filter, err := ParseQualityTrendFilter("", "", "ops", now, location)
	if err != nil {
		t.Fatalf("parse default filter returned error: %v", err)
	}
	if filter.SpaceID != "ops" || filter.AllSpaces || filter.Start.Format("2006-01-02") != "2026-06-12" || filter.End.Format("2006-01-02") != "2026-07-12" {
		t.Fatalf("filter = %#v", filter)
	}
	allSpaces, err := ParseQualityTrendFilter("2026-07-01", "2026-07-11", "", now, location)
	if err != nil || !allSpaces.AllSpaces {
		t.Fatalf("all-space filter = %#v, err=%v", allSpaces, err)
	}
	if _, err := ParseQualityTrendFilter("2026-01-01", "2026-04-02", "", now, location); err == nil {
		t.Fatal("expected oversized range to fail")
	}
	dstLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load DST location: %v", err)
	}
	if _, err := ParseQualityTrendFilter("2026-03-08", "2026-03-08", "", now, dstLocation); err != nil {
		t.Fatalf("one DST calendar day returned error: %v", err)
	}
}

func TestParseQualityTrendFilterIncludesReviewContext(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	filter, err := ParseQualityTrendFilter("2026-03-08", "2026-03-10", "ops", time.Date(2026, 3, 11, 12, 0, 0, 0, time.UTC), location)
	if err != nil {
		t.Fatalf("parse filter returned error: %v", err)
	}
	if filter.From != "2026-03-08" || filter.To != "2026-03-10" || filter.Timezone != "America/New_York" || filter.WindowDays != 3 {
		t.Fatalf("review context = %#v", filter)
	}
}

func TestQualityTrendServiceProjectsVerificationAndRecurrenceByTenantAndSpace(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	gapStore := NewInMemoryKnowledgeGapStore()
	gaps := NewKnowledgeGapService(gapStore)
	knowledge := NewKnowledgeService(NewInMemoryKnowledgeStore())
	if _, err := knowledge.CreateSpace("tenant-a", KnowledgeSpaceInput{ID: "ops", Name: "Operations"}); err != nil {
		t.Fatalf("create space returned error: %v", err)
	}
	verificationStore := NewInMemoryRepairVerificationStore()
	verification := RepairVerificationService{store: verificationStore, gaps: gaps, knowledge: knowledge}
	recurrenceStore := NewInMemoryRepairRecurrenceStore()
	observationStore := NewInMemoryRepairEvalObservationStore()
	currentSnapshot, err := repairVerificationSnapshotFingerprint(nil, "ops")
	if err != nil {
		t.Fatalf("snapshot fingerprint returned error: %v", err)
	}
	for index := 0; index < 3; index++ {
		gap := KnowledgeGap{ID: "gap-" + string(rune('a'+index)), TenantID: "tenant-a", SpaceID: "ops", Question: "question", NoSourceReason: "missing", Status: KnowledgeGapResolved, CreatedAt: now.Add(-time.Duration(index+1) * time.Hour)}
		if _, err := gapStore.SaveKnowledgeGap(gap); err != nil {
			t.Fatalf("save gap returned error: %v", err)
		}
		completed := now.Add(-time.Duration(index+1) * time.Hour).Add(time.Duration(index+1) * time.Minute)
		attempt := RepairVerificationAttempt{ID: "verification-" + string(rune('a'+index)), TenantID: "tenant-a", GapID: gap.ID, SpaceID: gap.SpaceID, StartedAt: completed.Add(-time.Minute), CompletedAt: completed, Result: RepairVerificationPassed, KnowledgeSnapshotFingerprint: currentSnapshot}
		if _, err := verificationStore.AppendRepairVerification(attempt); err != nil {
			t.Fatalf("append verification returned error: %v", err)
		}
		recurrence := RepairRecurrence{ID: "recurrence-" + string(rune('a'+index)), TenantID: "tenant-a", GapID: gap.ID, SpaceID: gap.SpaceID, Status: RepairRecurrenceSuspected, OccurrenceCount: index + 1, CreatedAt: completed, UpdatedAt: completed}
		if _, err := recurrenceStore.SaveRepairRecurrence(recurrence); err != nil {
			t.Fatalf("save recurrence returned error: %v", err)
		}
	}
	for index, status := range []RepairEvalObservationStatus{RepairEvalObservationPassed, RepairEvalObservationFailed, RepairEvalObservationUnavailable} {
		record := repairEvalObservation("tenant-a", "run-"+string(rune('a'+index)), "case-"+string(rune('a'+index)), "promotion-"+string(rune('a'+index)))
		record.GapID = "gap-" + string(rune('a'+index))
		record.SpaceID = "ops"
		record.ObservedAt = now.Add(-time.Duration(index+1) * time.Hour)
		record.Status = status
		if status != RepairEvalObservationPassed {
			record.FailureReason = "assertion_failed"
		}
		if _, _, err := observationStore.AppendRepairEvalObservation(record); err != nil {
			t.Fatalf("append eval observation returned error: %v", err)
		}
	}
	staleGap := KnowledgeGap{ID: "gap-stale", TenantID: "tenant-a", SpaceID: "ops", Question: "stale", NoSourceReason: "missing", Status: KnowledgeGapResolved, CreatedAt: now.Add(-100 * 24 * time.Hour)}
	if _, err := gapStore.SaveKnowledgeGap(staleGap); err != nil {
		t.Fatalf("save stale gap returned error: %v", err)
	}
	if _, err := verificationStore.AppendRepairVerification(RepairVerificationAttempt{ID: "verification-stale", TenantID: "tenant-a", GapID: staleGap.ID, SpaceID: staleGap.SpaceID, CompletedAt: now.Add(-100 * 24 * time.Hour), Result: RepairVerificationPassed, KnowledgeSnapshotFingerprint: "old-snapshot"}); err != nil {
		t.Fatalf("append stale verification returned error: %v", err)
	}
	otherTenant := KnowledgeGap{ID: "gap-other", TenantID: "tenant-b", SpaceID: "ops", Question: "other", NoSourceReason: "missing", Status: KnowledgeGapResolved, CreatedAt: now.Add(-time.Hour)}
	if _, err := gapStore.SaveKnowledgeGap(otherTenant); err != nil {
		t.Fatalf("save other gap returned error: %v", err)
	}
	service := NewQualityTrendService(QualityTrendDependencies{
		Gaps: gaps, Knowledge: knowledge, Verification: verification, Recurrences: recurrenceStore,
		Observations: observationStore, Now: func() time.Time { return now }, Location: location,
	})
	projection, err := service.Project("tenant-a", QualityTrendRequest{From: "2026-07-01", To: "2026-07-11", SpaceID: "ops"})
	if err != nil {
		t.Fatalf("project returned error: %v", err)
	}
	if !projection.ProjectedAt.Equal(now) {
		t.Fatalf("projected at = %s, want %s", projection.ProjectedAt, now)
	}
	if projection.Verification.AttemptCount != 3 || projection.Verification.PassedGapCount != 3 {
		t.Fatalf("verification = %#v", projection.Verification)
	}
	if projection.Recurrence.SuspectedCount != 3 || projection.Recurrence.Rate.Status != QualityTrendObserved || projection.Recurrence.Rate.Value == nil || *projection.Recurrence.Rate.Value != 1 {
		t.Fatalf("recurrence = %#v", projection.Recurrence)
	}
	if projection.PromotedEval.PassedCount != 1 || projection.PromotedEval.FailedCount != 1 || projection.PromotedEval.UnavailableCount != 1 || projection.PromotedEval.Rate.Value == nil || *projection.PromotedEval.Rate.Value != 1.0/3.0 {
		t.Fatalf("promoted eval = %#v", projection.PromotedEval)
	}
	if projection.Verification.Stale.Count != 1 || projection.Verification.Stale.Status != QualityTrendObserved {
		t.Fatalf("stale = %#v", projection.Verification.Stale)
	}
	if len(projection.RepeatedFailures.Questions) != 3 || projection.RepeatedFailures.Questions[0].GapID != "gap-c" || projection.RepeatedFailures.Questions[0].OccurrenceCount != 3 {
		t.Fatalf("repeated questions = %#v", projection.RepeatedFailures.Questions)
	}
	if len(projection.RepeatedFailures.Spaces) != 1 || projection.RepeatedFailures.Spaces[0].SpaceID != "ops" {
		t.Fatalf("repeated spaces = %#v", projection.RepeatedFailures.Spaces)
	}
	if len(projection.RepeatedFailures.PromotedCases) != 1 || projection.RepeatedFailures.PromotedCases[0].FailedCount != 1 {
		t.Fatalf("promoted failures = %#v", projection.RepeatedFailures.PromotedCases)
	}
	if len(projection.PromotedEval.Buckets) != 2 || projection.PromotedEval.Buckets[0].TotalCount != 0 || projection.PromotedEval.Buckets[1].TotalCount == 0 {
		t.Fatalf("eval buckets = %#v", projection.PromotedEval.Buckets)
	}
	if len(projection.Trace.GapIDs) != 3 || len(projection.Trace.RecurrenceIDs) != 3 {
		t.Fatalf("trace = %#v", projection.Trace)
	}
}

func TestQualityTrendServiceWithholdsSmallSamples(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	gapStore := NewInMemoryKnowledgeGapStore()
	gaps := NewKnowledgeGapService(gapStore)
	gap := KnowledgeGap{ID: "gap-1", TenantID: "tenant-a", SpaceID: DefaultKnowledgeSpaceID, Question: "question", NoSourceReason: "missing", Status: KnowledgeGapResolved, CreatedAt: now.Add(-time.Hour)}
	if _, err := gapStore.SaveKnowledgeGap(gap); err != nil {
		t.Fatalf("save gap returned error: %v", err)
	}
	verificationStore := NewInMemoryRepairVerificationStore()
	if _, err := verificationStore.AppendRepairVerification(RepairVerificationAttempt{ID: "verification-1", TenantID: "tenant-a", GapID: gap.ID, SpaceID: gap.SpaceID, CompletedAt: now.Add(-time.Minute), Result: RepairVerificationPassed}); err != nil {
		t.Fatalf("append verification returned error: %v", err)
	}
	knowledge := NewKnowledgeService(NewInMemoryKnowledgeStore())
	service := NewQualityTrendService(QualityTrendDependencies{Gaps: gaps, Knowledge: knowledge, Verification: RepairVerificationService{store: verificationStore, gaps: gaps, knowledge: knowledge}, Recurrences: NewInMemoryRepairRecurrenceStore(), Observations: NewInMemoryRepairEvalObservationStore(), Now: func() time.Time { return now }, Location: time.UTC})
	projection, err := service.Project("tenant-a", QualityTrendRequest{From: "2026-07-01", To: "2026-07-11"})
	if err != nil {
		t.Fatalf("project returned error: %v", err)
	}
	if projection.Recurrence.Rate.Status != QualityTrendInsufficientSample || projection.Recurrence.Rate.Value != nil {
		t.Fatalf("rate = %#v", projection.Recurrence.Rate)
	}
	if projection.Verification.TimeToVerify.Status != QualityTrendInsufficientSample || projection.Verification.TimeToVerify.MedianMS != nil {
		t.Fatalf("time to verify = %#v", projection.Verification.TimeToVerify)
	}
}

func TestQualityTrendServiceReturnsUnavailableWithoutKnowledgeDependency(t *testing.T) {
	gaps := NewKnowledgeGapService(NewInMemoryKnowledgeGapStore())
	verification := RepairVerificationService{store: NewInMemoryRepairVerificationStore()}
	service := NewQualityTrendService(QualityTrendDependencies{Gaps: gaps, Verification: verification, Recurrences: NewInMemoryRepairRecurrenceStore(), Observations: NewInMemoryRepairEvalObservationStore()})
	if _, err := service.Project("tenant-a", QualityTrendRequest{}); err == nil || err.Error() != "quality trend service unavailable" {
		t.Fatalf("project error = %v, want unavailable", err)
	}
}

func TestTruncateQualityTrendQuestionPreservesUTF8Boundaries(t *testing.T) {
	input := strings.Repeat("中", 161)
	if got := truncateQualityTrendQuestion(input); len([]rune(got)) != 160 {
		t.Fatalf("rune length = %d, want 160", len([]rune(got)))
	}
}

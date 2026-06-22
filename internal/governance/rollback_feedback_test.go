package governance

import (
	"testing"
	"time"
)

func TestRollbackServiceRecordsPreviousAndTargetVersions(t *testing.T) {
	store := NewInMemoryDecisionStore()
	service := RollbackService{Decisions: store}

	record, err := service.Record(RollbackRecord{
		ID:              "rollback-1",
		TenantID:        "tenant-1",
		CandidateType:   CandidatePersona,
		PreviousVersion: "persona-v2",
		TargetVersion:   "persona-v1",
		ActorID:         "operator-1",
		CreatedAt:       time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Record returned error: %v", err)
	}
	if record.PreviousVersion != "persona-v2" || record.TargetVersion != "persona-v1" {
		t.Fatalf("rollback record = %#v", record)
	}

	decisions, err := store.ListDecisions("tenant-1")
	if err != nil {
		t.Fatalf("ListDecisions: %v", err)
	}
	if len(decisions) != 1 || decisions[0].Type != DecisionRollback {
		t.Fatalf("rollback decision missing: %#v", decisions)
	}
}

func TestFeedbackServiceCreatesTriagesAndLinksFeedback(t *testing.T) {
	store := NewInMemoryFeedbackStore()
	service := FeedbackService{Store: store}

	record, err := service.Create(FeedbackRecord{
		ID:             "feedback-1",
		TenantID:       "tenant-1",
		ConversationID: "conv-1",
		MessageID:      "msg-1",
		Category:       FeedbackPersona,
		Severity:       FeedbackSeverityMedium,
		Note:           "Tone drifted from professional persona.",
		CreatedBy:      "operator-1",
		CreatedAt:      time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if record.Status != FeedbackNew {
		t.Fatalf("status = %q, want new", record.Status)
	}

	triaged, err := service.UpdateStatus("tenant-1", "feedback-1", FeedbackEvalAdded, "persona-disclosure")
	if err != nil {
		t.Fatalf("UpdateStatus returned error: %v", err)
	}
	if triaged.Status != FeedbackEvalAdded || triaged.LinkedEvalCaseID != "persona-disclosure" {
		t.Fatalf("triaged feedback = %#v", triaged)
	}

	tenantOne, err := store.ListFeedback("tenant-1")
	if err != nil {
		t.Fatalf("ListFeedback tenant-1: %v", err)
	}
	if len(tenantOne) != 1 {
		t.Fatalf("tenant feedback = %#v", tenantOne)
	}
	tenantTwo, err := store.ListFeedback("tenant-2")
	if err != nil {
		t.Fatalf("ListFeedback tenant-2: %v", err)
	}
	if len(tenantTwo) != 0 {
		t.Fatalf("feedback leaked across tenants: %#v", tenantTwo)
	}
}

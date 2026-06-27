package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPhase8TurnContractsRoundTripJSONAndValidation(t *testing.T) {
	createdAt := time.Date(2026, 6, 25, 9, 0, 0, 0, time.UTC)
	request := TurnRequest{
		ConversationID: "conv-1",
		TenantID:       "tenant-1",
		UserID:         "user-1",
		TurnID:         "turn-1",
		AttemptID:      "attempt-1",
		Message: Message{
			ID:        "msg-1",
			Role:      RoleUser,
			Content:   "Tell me what we discussed yesterday.",
			CreatedAt: createdAt,
		},
		Metadata: Metadata{"source": "test"},
	}

	if err := request.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	var decodedRequest TurnRequest
	roundTripJSON(t, request, &decodedRequest)
	if decodedRequest.TurnID != request.TurnID || decodedRequest.AttemptID != request.AttemptID {
		t.Fatalf("turn request round trip = %#v", decodedRequest)
	}

	record := TurnRecord{
		ID:                 "turn-1",
		UserMessageID:      "msg-1",
		AssistantMessageID: "msg-2",
		Status:             TurnCompleted,
		Attempts: []TurnAttempt{{
			ID:          "attempt-1",
			Status:      AttemptCompleted,
			RequestID:   "req-1",
			StartedAt:   createdAt,
			CompletedAt: createdAt.Add(2 * time.Second),
		}},
		Result: &AgentResult{
			AgentName: "persona-agent",
			Message: Message{
				ID:        "msg-2",
				Role:      RoleAssistant,
				Content:   "We discussed Phase 8 streaming behavior.",
				CreatedAt: createdAt.Add(2 * time.Second),
			},
			Confidence: Confidence(0.9),
		},
	}

	if err := record.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	var decodedRecord TurnRecord
	roundTripJSON(t, record, &decodedRecord)
	if decodedRecord.Status != TurnCompleted || len(decodedRecord.Attempts) != 1 {
		t.Fatalf("turn record round trip = %#v", decodedRecord)
	}

	event := StreamEvent{
		Name:           StreamEventMessageCompleted,
		RequestID:      "req-1",
		TenantID:       "tenant-1",
		UserID:         "user-1",
		ConversationID: "conv-1",
		TurnID:         "turn-1",
		AttemptID:      "attempt-1",
		Sequence:       4,
		Timestamp:      createdAt.Add(2 * time.Second),
		Payload:        Metadata{"agent": "persona-agent"},
		Metadata:       Metadata{"replayed": false},
	}

	if err := event.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	var decodedEvent StreamEvent
	roundTripJSON(t, event, &decodedEvent)
	if decodedEvent.Name != StreamEventMessageCompleted || decodedEvent.Sequence != 4 {
		t.Fatalf("stream event round trip = %#v", decodedEvent)
	}
}

func TestPhase8TurnRequestRejectsInvalidInputs(t *testing.T) {
	base := TurnRequest{
		ConversationID: "conv-1",
		TenantID:       "tenant-1",
		UserID:         "user-1",
		TurnID:         "turn-1",
		AttemptID:      "attempt-1",
		Message: Message{
			ID:      "msg-1",
			Role:    RoleUser,
			Content: "hello",
		},
	}

	tests := []struct {
		name   string
		mutate func(*TurnRequest)
	}{
		{name: "missing conversation id", mutate: func(req *TurnRequest) { req.ConversationID = "" }},
		{name: "unsafe turn id", mutate: func(req *TurnRequest) { req.TurnID = "../escape" }},
		{name: "unsafe attempt id", mutate: func(req *TurnRequest) { req.AttemptID = "attempt/1" }},
		{name: "assistant role", mutate: func(req *TurnRequest) { req.Message.Role = RoleAssistant }},
		{name: "blank content", mutate: func(req *TurnRequest) { req.Message.Content = "   " }},
		{name: "missing message id", mutate: func(req *TurnRequest) { req.Message.ID = "" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := base
			tt.mutate(&req)
			if err := req.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want invalid input")
			}
		})
	}
}

func TestPhase8TurnAndAttemptStatusesAreStable(t *testing.T) {
	if string(TurnOpen) != "open" || string(TurnCompleted) != "completed" || string(TurnFailed) != "failed" || string(TurnCanceled) != "canceled" {
		t.Fatalf("unexpected turn status values")
	}
	if string(AttemptGenerating) != "generating" ||
		string(AttemptCompleted) != "completed" ||
		string(AttemptFailed) != "failed" ||
		string(AttemptCanceled) != "canceled" ||
		string(AttemptAbandoned) != "abandoned" ||
		string(AttemptReplayed) != "replayed" {
		t.Fatalf("unexpected attempt status values")
	}
}

func TestPhase8StreamEventNamesAreStable(t *testing.T) {
	names := map[StreamEventName]string{
		StreamEventRequestStarted:   "request_started",
		StreamEventRouteSelected:    "route_selected",
		StreamEventAgentSelected:    "agent_selected",
		StreamEventAssistantDelta:   "assistant_text_delta",
		StreamEventFallbackSelected: "fallback_selected",
		StreamEventMessageCompleted: "message_completed",
		StreamEventCanceled:         "canceled",
		StreamEventError:            "error",
		StreamEventDone:             "done",
	}

	for got, want := range names {
		if string(got) != want {
			t.Fatalf("event name = %q, want %q", got, want)
		}
	}
}

func TestConversationContractsRoundTripJSON(t *testing.T) {
	createdAt := time.Date(2026, 6, 14, 10, 30, 0, 0, time.UTC)
	conversation := Conversation{
		ID:       "conv-1",
		TenantID: "tenant-1",
		UserID:   "user-1",
		Messages: []Message{
			{
				ID:        "msg-1",
				Role:      RoleSystem,
				Content:   "You are a professional digital twin.",
				CreatedAt: createdAt,
				Metadata:  Metadata{"source": "test"},
			},
			{
				ID:        "msg-2",
				Role:      RoleUser,
				Content:   "hello",
				CreatedAt: createdAt.Add(time.Minute),
			},
		},
		Metadata:  Metadata{"topic": "contracts"},
		CreatedAt: createdAt,
		UpdatedAt: createdAt.Add(time.Minute),
	}

	var decoded Conversation
	roundTripJSON(t, conversation, &decoded)

	if decoded.ID != conversation.ID {
		t.Fatalf("expected ID %q, got %q", conversation.ID, decoded.ID)
	}
	if len(decoded.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(decoded.Messages))
	}
	if decoded.Messages[0].Role != RoleSystem {
		t.Fatalf("expected first role %q, got %q", RoleSystem, decoded.Messages[0].Role)
	}
	if decoded.Messages[0].Metadata["source"] != "test" {
		t.Fatalf("expected metadata to round trip")
	}
}

func TestAgentAndSkillResultsRoundTripJSON(t *testing.T) {
	agent := AgentResult{
		AgentName:  "memory-agent",
		Message:    Message{ID: "msg-3", Role: RoleAssistant, Content: "remembered"},
		Confidence: Confidence(0.92),
		Metadata:   Metadata{"trace": "abc"},
	}

	var decodedAgent AgentResult
	roundTripJSON(t, agent, &decodedAgent)

	if decodedAgent.AgentName != agent.AgentName {
		t.Fatalf("expected agent name %q, got %q", agent.AgentName, decodedAgent.AgentName)
	}
	if !decodedAgent.Confidence.Valid() {
		t.Fatalf("expected confidence to be valid")
	}

	skill := SkillResult{
		SkillName: "knowledge.search",
		Output:    map[string]any{"answer": "ok"},
		Metadata:  Metadata{"latency_ms": float64(10)},
	}

	var decodedSkill SkillResult
	roundTripJSON(t, skill, &decodedSkill)

	if decodedSkill.SkillName != skill.SkillName {
		t.Fatalf("expected skill name %q, got %q", skill.SkillName, decodedSkill.SkillName)
	}
	if decodedSkill.Output == nil {
		t.Fatalf("expected output to round trip")
	}
}

func TestIntentUserProfileAndTenantRoundTripJSON(t *testing.T) {
	intent := Intent{
		Name:       IntentKnowledgeQuery,
		Query:      "what do I know?",
		Confidence: Confidence(1),
		Entities:   Metadata{"subject": "memory"},
	}
	profile := UserProfile{
		ID:          "user-1",
		DisplayName: "Ada",
		Locale:      "zh-CN",
		Timezone:    "Asia/Shanghai",
		Metadata:    Metadata{"tier": "pro"},
	}
	tenant := Tenant{
		ID:       "tenant-1",
		Name:     "Digital Twin Lab",
		Metadata: Metadata{"region": "local"},
	}

	var decodedIntent Intent
	roundTripJSON(t, intent, &decodedIntent)
	if decodedIntent.Name != IntentKnowledgeQuery {
		t.Fatalf("expected intent %q, got %q", IntentKnowledgeQuery, decodedIntent.Name)
	}

	var decodedProfile UserProfile
	roundTripJSON(t, profile, &decodedProfile)
	if decodedProfile.Locale != "zh-CN" {
		t.Fatalf("expected locale to round trip")
	}

	var decodedTenant Tenant
	roundTripJSON(t, tenant, &decodedTenant)
	if decodedTenant.Name != tenant.Name {
		t.Fatalf("expected tenant name to round trip")
	}
}

func TestRoleAndConfidenceValidation(t *testing.T) {
	if !RoleUser.Valid() || !RoleAssistant.Valid() || !RoleSystem.Valid() || !RoleTool.Valid() {
		t.Fatalf("expected known roles to be valid")
	}
	if Role("ghost").Valid() {
		t.Fatalf("did not expect unknown role to be valid")
	}

	for _, value := range []Confidence{0, 0.5, 1} {
		if !value.Valid() {
			t.Fatalf("expected confidence %v to be valid", value)
		}
	}
	for _, value := range []Confidence{-0.1, 1.1} {
		if value.Valid() {
			t.Fatalf("did not expect confidence %v to be valid", value)
		}
	}
}

func TestPhase2IntentNamesAreStable(t *testing.T) {
	tests := map[IntentName]string{
		IntentPersonaChat: "persona.chat",
		IntentSafetyCheck: "safety.check",
	}

	for got, want := range tests {
		if string(got) != want {
			t.Fatalf("intent name = %q, want %q", got, want)
		}
	}
}

func roundTripJSON[T any](t *testing.T, value T, out *T) {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

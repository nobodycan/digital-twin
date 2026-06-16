package types

import (
	"encoding/json"
	"testing"
	"time"
)

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

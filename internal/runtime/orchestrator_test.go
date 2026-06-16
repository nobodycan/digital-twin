package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestOrchestratorRoutesToAgentAndRecordsEvents(t *testing.T) {
	conversation := validRuntimeConversation("plan my week")
	intent := types.Intent{Name: types.IntentTaskExecution, Query: "plan my week", Confidence: 0.9}
	agentResult := types.AgentResult{
		AgentName: "task-agent",
		Message: types.Message{
			Role:    types.RoleAssistant,
			Content: "I broke the task into steps.",
		},
		Confidence: 0.9,
	}
	agents := core.NewAgentRegistry()
	if err := agents.Register(fakeRuntimeAgent{name: "task-agent", handles: types.IntentTaskExecution, result: agentResult}); err != nil {
		t.Fatalf("register agent: %v", err)
	}
	recorder := NewEventRecorder()
	orchestrator := NewOrchestrator(OrchestratorConfig{
		Router:    &fakeRuntimeRouter{intent: intent},
		Agents:    agents,
		Recorder:  recorder,
		RequestID: func() string { return "req-1" },
	})

	result, err := orchestrator.Handle(t.Context(), conversation)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if result.AgentName != "task-agent" || result.Message.Content != "I broke the task into steps." {
		t.Fatalf("Handle() result = %#v", result)
	}

	events := recorder.Events()
	wantTopics := []string{"request_started", "state_changed", "route_selected", "agent_selected", "state_changed", "message_completed", "done"}
	if len(events) != len(wantTopics) {
		t.Fatalf("events len = %d, want %d: %#v", len(events), len(wantTopics), events)
	}
	for i, want := range wantTopics {
		if events[i].Topic != want {
			t.Fatalf("events[%d].Topic = %q, want %q", i, events[i].Topic, want)
		}
		if events[i].RequestID != "req-1" || events[i].ConversationID != conversation.ID {
			t.Fatalf("events[%d] correlation = %#v", i, events[i])
		}
	}
}

func TestOrchestratorFallsBackToPersonaIntentWhenRouterFails(t *testing.T) {
	conversation := validRuntimeConversation("hello")
	agents := core.NewAgentRegistry()
	if err := agents.Register(fakeRuntimeAgent{
		name:    "persona-agent",
		handles: types.IntentPersonaChat,
		result: types.AgentResult{
			AgentName:  "persona-agent",
			Message:    types.Message{Role: types.RoleAssistant, Content: "safe fallback"},
			Confidence: 0.3,
		},
	}); err != nil {
		t.Fatalf("register agent: %v", err)
	}
	recorder := NewEventRecorder()
	orchestrator := NewOrchestrator(OrchestratorConfig{
		Router:    &fakeRuntimeRouter{err: fmt.Errorf("classifier unavailable")},
		Agents:    agents,
		Recorder:  recorder,
		RequestID: func() string { return "req-fallback" },
	})

	result, err := orchestrator.Handle(t.Context(), conversation)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if result.AgentName != "persona-agent" || result.Message.Content != "safe fallback" {
		t.Fatalf("Handle() result = %#v", result)
	}
	events := recorder.Events()
	if !hasRecordedTopic(events, string(EventRuntimeError)) {
		t.Fatalf("events missing runtime_error: %#v", events)
	}
	if !hasRecordedTopic(events, string(EventRouteSelected)) {
		t.Fatalf("events missing route_selected fallback: %#v", events)
	}
}

func TestOrchestratorFallsBackToPersonaIntentForLowConfidenceRoute(t *testing.T) {
	conversation := validRuntimeConversation("maybe do something")
	agents := core.NewAgentRegistry()
	if err := agents.Register(fakeRuntimeAgent{
		name:    "persona-agent",
		handles: types.IntentPersonaChat,
		result:  types.AgentResult{AgentName: "persona-agent", Message: types.Message{Role: types.RoleAssistant, Content: "fallback"}, Confidence: 0.3},
	}); err != nil {
		t.Fatalf("register agent: %v", err)
	}
	orchestrator := NewOrchestrator(OrchestratorConfig{
		Router: &fakeRuntimeRouter{intent: types.Intent{
			Name:       types.IntentTaskExecution,
			Query:      "maybe do something",
			Confidence: 0.2,
		}},
		Agents:   agents,
		Recorder: NewEventRecorder(),
	})

	result, err := orchestrator.Handle(t.Context(), conversation)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if result.AgentName != "persona-agent" {
		t.Fatalf("AgentName = %q, want persona-agent", result.AgentName)
	}
}

func TestOrchestratorReturnsSafeResultWhenAgentIsMissing(t *testing.T) {
	conversation := validRuntimeConversation("search knowledge")
	orchestrator := NewOrchestrator(OrchestratorConfig{
		Router:   &fakeRuntimeRouter{intent: types.Intent{Name: types.IntentKnowledgeQuery, Query: "search knowledge", Confidence: 0.9}},
		Agents:   core.NewAgentRegistry(),
		Recorder: NewEventRecorder(),
	})

	result, err := orchestrator.Handle(t.Context(), conversation)
	if err != nil {
		t.Fatalf("Handle() error = %v, want safe result", err)
	}
	if result.AgentName != "runtime-fallback" {
		t.Fatalf("AgentName = %q, want runtime-fallback", result.AgentName)
	}
	if result.Metadata["error"] != "agent_not_found" {
		t.Fatalf("Metadata[error] = %#v, want agent_not_found", result.Metadata["error"])
	}
}

func TestOrchestratorReturnsSafeResultWhenAgentFails(t *testing.T) {
	conversation := validRuntimeConversation("plan")
	agents := core.NewAgentRegistry()
	if err := agents.Register(fakeRuntimeAgent{
		name:    "task-agent",
		handles: types.IntentTaskExecution,
		err:     fmt.Errorf("dependency failed"),
	}); err != nil {
		t.Fatalf("register agent: %v", err)
	}
	orchestrator := NewOrchestrator(OrchestratorConfig{
		Router:   &fakeRuntimeRouter{intent: types.Intent{Name: types.IntentTaskExecution, Query: "plan", Confidence: 0.8}},
		Agents:   agents,
		Recorder: NewEventRecorder(),
	})

	result, err := orchestrator.Handle(t.Context(), conversation)
	if err != nil {
		t.Fatalf("Handle() error = %v, want safe result", err)
	}
	if result.AgentName != "runtime-fallback" {
		t.Fatalf("AgentName = %q, want runtime-fallback", result.AgentName)
	}
	if result.Metadata["error"] != "agent_failed" || result.Metadata["agent"] != "task-agent" {
		t.Fatalf("Metadata = %#v, want agent_failed for task-agent", result.Metadata)
	}
}

func TestOrchestratorReturnsContextErrorWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	orchestrator := NewOrchestrator(OrchestratorConfig{
		Router:   &fakeRuntimeRouter{intent: types.Intent{Name: types.IntentPersonaChat, Query: "hello", Confidence: 0.8}},
		Agents:   core.NewAgentRegistry(),
		Recorder: NewEventRecorder(),
	})

	_, err := orchestrator.Handle(ctx, validRuntimeConversation("hello"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Handle() error = %v, want context.Canceled", err)
	}
}

func TestOrchestratorRecoversAgentPanic(t *testing.T) {
	conversation := validRuntimeConversation("plan")
	agents := core.NewAgentRegistry()
	if err := agents.Register(fakeRuntimeAgent{
		name:    "task-agent",
		handles: types.IntentTaskExecution,
		panic:   "boom",
	}); err != nil {
		t.Fatalf("register agent: %v", err)
	}
	recorder := NewEventRecorder()
	orchestrator := NewOrchestrator(OrchestratorConfig{
		Router:   &fakeRuntimeRouter{intent: types.Intent{Name: types.IntentTaskExecution, Query: "plan", Confidence: 0.8}},
		Agents:   agents,
		Recorder: recorder,
	})

	_, err := orchestrator.Handle(t.Context(), conversation)
	if err == nil {
		t.Fatal("Handle() error = nil, want recovered panic error")
	}
	if !errors.Is(err, core.ErrProviderFailure) {
		t.Fatalf("Handle() error = %v, want ErrProviderFailure", err)
	}
	if !hasRecordedTopic(recorder.Events(), string(EventRuntimeError)) {
		t.Fatalf("events missing runtime_error: %#v", recorder.Events())
	}
}

func TestOrchestratorGeneratesUniqueRequestIDsForConcurrentRequests(t *testing.T) {
	agents := core.NewAgentRegistry()
	if err := agents.Register(fakeRuntimeAgent{
		name:    "persona-agent",
		handles: types.IntentPersonaChat,
		result:  types.AgentResult{AgentName: "persona-agent", Message: types.Message{Role: types.RoleAssistant, Content: "ok"}, Confidence: 0.8},
	}); err != nil {
		t.Fatalf("register agent: %v", err)
	}
	recorder := NewEventRecorder()
	orchestrator := NewOrchestrator(OrchestratorConfig{
		Router:   &fakeRuntimeRouter{intent: types.Intent{Name: types.IntentPersonaChat, Query: "hello", Confidence: 0.8}},
		Agents:   agents,
		Recorder: recorder,
	})

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			conversation := validRuntimeConversation(fmt.Sprintf("hello %d", i))
			conversation.ID = fmt.Sprintf("conv-%d", i)
			if _, err := orchestrator.Handle(t.Context(), conversation); err != nil {
				t.Errorf("Handle(%d) error = %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	seen := map[string]bool{}
	for _, event := range recorder.Events() {
		if event.Topic != string(EventRequestStarted) {
			continue
		}
		if event.RequestID == "" {
			t.Fatal("request_started event has empty RequestID")
		}
		if seen[event.RequestID] {
			t.Fatalf("duplicate RequestID %q", event.RequestID)
		}
		seen[event.RequestID] = true
	}
	if len(seen) != 8 {
		t.Fatalf("request IDs = %d, want 8", len(seen))
	}
}

func TestOrchestratorRejectsInvalidConversationBeforeRouting(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*types.Conversation)
	}{
		{
			name: "missing conversation id",
			mutate: func(c *types.Conversation) {
				c.ID = ""
			},
		},
		{
			name: "missing tenant id",
			mutate: func(c *types.Conversation) {
				c.TenantID = ""
			},
		},
		{
			name: "missing user id",
			mutate: func(c *types.Conversation) {
				c.UserID = ""
			},
		},
		{
			name: "missing user message",
			mutate: func(c *types.Conversation) {
				c.Messages = nil
			},
		},
		{
			name: "blank user message",
			mutate: func(c *types.Conversation) {
				c.Messages[0].Content = "   "
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conversation := validRuntimeConversation("hello")
			tt.mutate(&conversation)
			router := fakeRuntimeRouter{intent: types.Intent{Name: types.IntentPersonaChat, Query: "hello", Confidence: 0.8}}
			orchestrator := NewOrchestrator(OrchestratorConfig{
				Router:   &router,
				Agents:   core.NewAgentRegistry(),
				Recorder: NewEventRecorder(),
			})

			_, err := orchestrator.Handle(t.Context(), conversation)
			if err == nil {
				t.Fatal("Handle() error = nil, want invalid input")
			}
			if !errors.Is(err, core.ErrInvalidInput) {
				t.Fatalf("Handle() error = %v, want ErrInvalidInput", err)
			}
			if router.called {
				t.Fatal("router was called for invalid conversation")
			}
		})
	}
}

func validRuntimeConversation(content string) types.Conversation {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	return types.Conversation{
		ID:        "conv-1",
		TenantID:  "tenant-1",
		UserID:    "user-1",
		CreatedAt: now,
		UpdatedAt: now,
		Messages: []types.Message{{
			ID:        "msg-1",
			Role:      types.RoleUser,
			Content:   content,
			CreatedAt: now,
		}},
	}
}

type fakeRuntimeRouter struct {
	intent types.Intent
	err    error
	called bool
}

func (r *fakeRuntimeRouter) Route(context.Context, types.Conversation) (types.Intent, error) {
	r.called = true
	return r.intent, r.err
}

type fakeRuntimeAgent struct {
	name    string
	handles types.IntentName
	result  types.AgentResult
	err     error
	panic   any
}

func (a fakeRuntimeAgent) Name() string { return a.name }

func (a fakeRuntimeAgent) CanHandle(intent types.Intent) bool {
	return intent.Name == a.handles
}

func (a fakeRuntimeAgent) Run(context.Context, types.Conversation, types.Intent) (types.AgentResult, error) {
	if a.panic != nil {
		panic(a.panic)
	}
	return a.result, a.err
}

func hasRecordedTopic(events []RuntimeEvent, topic string) bool {
	for _, event := range events {
		if event.Topic == topic {
			return true
		}
	}
	return false
}

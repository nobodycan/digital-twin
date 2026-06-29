package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/conversation"
	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/internal/store"
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
			if router.WasCalled() {
				t.Fatal("router was called for invalid conversation")
			}
		})
	}
}

func TestOrchestratorStreamEmitsOrderedRuntimeEvents(t *testing.T) {
	agents := core.NewAgentRegistry()
	if err := agents.Register(fakeStreamingRuntimeAgent{
		name:    "persona-agent",
		handles: types.IntentPersonaChat,
		stream: func(ctx context.Context, conversation types.Conversation, intent types.Intent, sink core.AssistantDeltaSink) (types.AgentResult, error) {
			if err := sink.EmitAssistantDelta(ctx, "Hello there."); err != nil {
				return types.AgentResult{}, err
			}
			return types.AgentResult{
				AgentName: "persona-agent",
				Message:   types.Message{ID: "msg-assistant-1", Role: types.RoleAssistant, Content: "Hello there."},
			}, nil
		},
	}); err != nil {
		t.Fatalf("register agent: %v", err)
	}

	orchestrator := NewOrchestrator(OrchestratorConfig{
		Router:      &fakeRuntimeRouter{intent: types.Intent{Name: types.IntentPersonaChat, Query: "hello", Confidence: 0.9}},
		Agents:      agents,
		Recorder:    NewEventRecorder(),
		RequestID:   func() string { return "req-stream-1" },
		Coordinator: conversation.NewCoordinator(conversation.CoordinatorConfig{Store: store.NewInMemoryStore()}),
	})

	sink := &recordingRuntimeStreamSink{}
	result, err := orchestrator.Stream(t.Context(), validTurnRequest("turn-1", "attempt-1", "hello"), sink)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	if result.Message.Content != "Hello there." {
		t.Fatalf("result content = %q, want streamed reply", result.Message.Content)
	}
	want := []types.StreamEventName{
		types.StreamEventRequestStarted,
		types.StreamEventRouteSelected,
		types.StreamEventAgentSelected,
		types.StreamEventAssistantDelta,
		types.StreamEventMessageCompleted,
		types.StreamEventDone,
	}
	if len(sink.events) != len(want) {
		t.Fatalf("events len = %d, want %d", len(sink.events), len(want))
	}
	for i, name := range want {
		if sink.events[i].Name != name {
			t.Fatalf("events[%d].Name = %q, want %q", i, sink.events[i].Name, name)
		}
		if sink.events[i].Sequence != uint64(i+1) {
			t.Fatalf("events[%d].Sequence = %d, want %d", i, sink.events[i].Sequence, i+1)
		}
	}
	if sink.events[3].Payload["content"] != "Hello there." {
		t.Fatalf("delta payload = %#v, want content", sink.events[3].Payload)
	}
}

func TestOrchestratorStreamSupportsLegacyAgentsWithoutDeltaEvents(t *testing.T) {
	agents := core.NewAgentRegistry()
	if err := agents.Register(fakeRuntimeAgent{
		name:    "persona-agent",
		handles: types.IntentPersonaChat,
		result: types.AgentResult{
			AgentName: "persona-agent",
			Message:   types.Message{ID: "msg-assistant-1", Role: types.RoleAssistant, Content: "Complete reply"},
		},
	}); err != nil {
		t.Fatalf("register agent: %v", err)
	}

	orchestrator := NewOrchestrator(OrchestratorConfig{
		Router:      &fakeRuntimeRouter{intent: types.Intent{Name: types.IntentPersonaChat, Query: "hello", Confidence: 0.9}},
		Agents:      agents,
		Recorder:    NewEventRecorder(),
		RequestID:   func() string { return "req-stream-2" },
		Coordinator: conversation.NewCoordinator(conversation.CoordinatorConfig{Store: store.NewInMemoryStore()}),
	})

	sink := &recordingRuntimeStreamSink{}
	result, err := orchestrator.Stream(t.Context(), validTurnRequest("turn-1", "attempt-1", "hello"), sink)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	if result.Message.Content != "Complete reply" {
		t.Fatalf("result content = %q, want complete reply", result.Message.Content)
	}
	if hasStreamEvent(sink.events, types.StreamEventAssistantDelta) {
		t.Fatalf("events = %#v, want no assistant_text_delta for legacy agent", sink.events)
	}
	if !hasStreamEvent(sink.events, types.StreamEventMessageCompleted) || !hasStreamEvent(sink.events, types.StreamEventDone) {
		t.Fatalf("events = %#v, want terminal events", sink.events)
	}
}

func TestOrchestratorStreamIncludesGenerationMetadataOnMessageCompleted(t *testing.T) {
	agents := core.NewAgentRegistry()
	if err := agents.Register(fakeStreamingRuntimeAgent{
		name:    "persona-agent",
		handles: types.IntentPersonaChat,
		stream: func(context.Context, types.Conversation, types.Intent, core.AssistantDeltaSink) (types.AgentResult, error) {
			return types.AgentResult{
				AgentName: "persona-agent",
				Message:   types.Message{ID: "msg-assistant-1", Role: types.RoleAssistant, Content: "Fallback reply"},
				Metadata: types.Metadata{
					"generation_mode":   "fallback",
					"fallback_category": "provider_status",
					"llm_provider":      "deepseek",
					"llm_model":         "deepseek-v4-pro",
					"api_key":           "should-not-leak",
				},
			}, nil
		},
	}); err != nil {
		t.Fatalf("register agent: %v", err)
	}

	orchestrator := NewOrchestrator(OrchestratorConfig{
		Router:      &fakeRuntimeRouter{intent: types.Intent{Name: types.IntentPersonaChat, Query: "hello", Confidence: 0.9}},
		Agents:      agents,
		Recorder:    NewEventRecorder(),
		RequestID:   func() string { return "req-stream-meta" },
		Coordinator: conversation.NewCoordinator(conversation.CoordinatorConfig{Store: store.NewInMemoryStore()}),
	})

	sink := &recordingRuntimeStreamSink{}
	if _, err := orchestrator.Stream(t.Context(), validTurnRequest("turn-1", "attempt-1", "hello"), sink); err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	event := findStreamEvent(t, sink.events, types.StreamEventMessageCompleted)
	for key, want := range map[string]any{
		"generation_mode":   "fallback",
		"fallback_category": "provider_status",
		"llm_provider":      "deepseek",
		"llm_model":         "deepseek-v4-pro",
	} {
		if event.Metadata[key] != want {
			t.Fatalf("metadata[%q] = %v, want %v", key, event.Metadata[key], want)
		}
	}
	if _, exists := event.Metadata["api_key"]; exists {
		t.Fatalf("message_completed metadata leaked api_key: %#v", event.Metadata)
	}
}

func TestOrchestratorStreamReplaysCompletedTurnWithoutCallingAgent(t *testing.T) {
	store := store.NewInMemoryStore()
	coordinator := conversation.NewCoordinator(conversation.CoordinatorConfig{Store: store})
	firstAgents := core.NewAgentRegistry()
	streamingAgent := fakeStreamingRuntimeAgent{
		name:    "persona-agent",
		handles: types.IntentPersonaChat,
		stream: func(ctx context.Context, conversation types.Conversation, intent types.Intent, sink core.AssistantDeltaSink) (types.AgentResult, error) {
			if err := sink.EmitAssistantDelta(ctx, "Hello there."); err != nil {
				return types.AgentResult{}, err
			}
			return types.AgentResult{
				AgentName: "persona-agent",
				Message:   types.Message{ID: "msg-assistant-1", Role: types.RoleAssistant, Content: "Hello there."},
			}, nil
		},
	}
	if err := firstAgents.Register(streamingAgent); err != nil {
		t.Fatalf("register first agent: %v", err)
	}
	first := NewOrchestrator(OrchestratorConfig{
		Router:      &fakeRuntimeRouter{intent: types.Intent{Name: types.IntentPersonaChat, Query: "hello", Confidence: 0.9}},
		Agents:      firstAgents,
		Recorder:    NewEventRecorder(),
		RequestID:   func() string { return "req-stream-first" },
		Coordinator: coordinator,
	})
	if _, err := first.Stream(t.Context(), validTurnRequest("turn-1", "attempt-1", "hello"), &recordingRuntimeStreamSink{}); err != nil {
		t.Fatalf("first Stream() error = %v", err)
	}

	replayAgents := core.NewAgentRegistry()
	replayAgent := fakeStreamingRuntimeAgent{
		name:    "persona-agent",
		handles: types.IntentPersonaChat,
		stream: func(context.Context, types.Conversation, types.Intent, core.AssistantDeltaSink) (types.AgentResult, error) {
			return types.AgentResult{}, errors.New("should not be called")
		},
	}
	if err := replayAgents.Register(replayAgent); err != nil {
		t.Fatalf("register replay agent: %v", err)
	}
	replay := NewOrchestrator(OrchestratorConfig{
		Router:      &fakeRuntimeRouter{intent: types.Intent{Name: types.IntentPersonaChat, Query: "hello", Confidence: 0.9}},
		Agents:      replayAgents,
		Recorder:    NewEventRecorder(),
		RequestID:   func() string { return "req-stream-replay" },
		Coordinator: coordinator,
	})

	sink := &recordingRuntimeStreamSink{}
	result, err := replay.Stream(t.Context(), validTurnRequest("turn-1", "attempt-2", "hello"), sink)
	if err != nil {
		t.Fatalf("replay Stream() error = %v", err)
	}
	if result.Message.Content != "Hello there." {
		t.Fatalf("replay result content = %q, want stored reply", result.Message.Content)
	}
	if hasStreamEvent(sink.events, types.StreamEventAssistantDelta) {
		t.Fatalf("events = %#v, want no delta on replay", sink.events)
	}
	if sink.events[len(sink.events)-2].Name != types.StreamEventMessageCompleted {
		t.Fatalf("events = %#v, want message_completed before done", sink.events)
	}
	if sink.events[len(sink.events)-2].Metadata["replayed"] != true {
		t.Fatalf("message_completed metadata = %#v, want replayed=true", sink.events[len(sink.events)-2].Metadata)
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
	mu     sync.Mutex
	intent types.Intent
	err    error
	called bool
}

func (r *fakeRuntimeRouter) Route(context.Context, types.Conversation) (types.Intent, error) {
	r.mu.Lock()
	r.called = true
	r.mu.Unlock()
	return r.intent, r.err
}

func (r *fakeRuntimeRouter) WasCalled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.called
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

type fakeStreamingRuntimeAgent struct {
	name    string
	handles types.IntentName
	stream  func(context.Context, types.Conversation, types.Intent, core.AssistantDeltaSink) (types.AgentResult, error)
}

func (a fakeStreamingRuntimeAgent) Name() string { return a.name }

func (a fakeStreamingRuntimeAgent) CanHandle(intent types.Intent) bool {
	return intent.Name == a.handles
}

func (a fakeStreamingRuntimeAgent) Run(ctx context.Context, conversation types.Conversation, intent types.Intent) (types.AgentResult, error) {
	return a.stream(ctx, conversation, intent, nil)
}

func (a fakeStreamingRuntimeAgent) Stream(ctx context.Context, conversation types.Conversation, intent types.Intent, sink core.AssistantDeltaSink) (types.AgentResult, error) {
	return a.stream(ctx, conversation, intent, sink)
}

type recordingRuntimeStreamSink struct {
	events []types.StreamEvent
}

func (s *recordingRuntimeStreamSink) Emit(_ context.Context, event types.StreamEvent) error {
	s.events = append(s.events, event)
	return nil
}

func hasStreamEvent(events []types.StreamEvent, name types.StreamEventName) bool {
	for _, event := range events {
		if event.Name == name {
			return true
		}
	}
	return false
}

func findStreamEvent(t *testing.T, events []types.StreamEvent, name types.StreamEventName) types.StreamEvent {
	t.Helper()
	for _, event := range events {
		if event.Name == name {
			return event
		}
	}
	t.Fatalf("missing stream event %q in %#v", name, events)
	return types.StreamEvent{}
}

func validTurnRequest(turnID, attemptID, content string) types.TurnRequest {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	return types.TurnRequest{
		ConversationID: "conv-stream",
		TenantID:       "tenant-1",
		UserID:         "user-1",
		TurnID:         turnID,
		AttemptID:      attemptID,
		Message: types.Message{
			ID:        "msg-" + turnID,
			Role:      types.RoleUser,
			Content:   content,
			CreatedAt: now,
		},
	}
}

func hasRecordedTopic(events []RuntimeEvent, topic string) bool {
	for _, event := range events {
		if event.Topic == topic {
			return true
		}
	}
	return false
}

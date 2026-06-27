package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/nobodycan/digital-twin/internal/conversation"
	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/pkg/types"
)

var requestCounter atomic.Uint64

type OrchestratorConfig struct {
	Router    core.Router
	Agents    *core.AgentRegistry
	Recorder  *EventRecorder
	RequestID func() string
	Coordinator *conversation.Coordinator
}

type Orchestrator struct {
	router    core.Router
	agents    *core.AgentRegistry
	recorder  *EventRecorder
	requestID func() string
	coordinator *conversation.Coordinator
}

func NewOrchestrator(config OrchestratorConfig) Orchestrator {
	requestID := config.RequestID
	if requestID == nil {
		requestID = func() string {
			return fmt.Sprintf("req-%d", requestCounter.Add(1))
		}
	}
	return Orchestrator{
		router:    config.Router,
		agents:    config.Agents,
		recorder:  config.Recorder,
		requestID: requestID,
		coordinator: config.Coordinator,
	}
}

func (o Orchestrator) Handle(ctx context.Context, conversation types.Conversation) (result types.AgentResult, err error) {
	if err := validateConversation(conversation); err != nil {
		return types.AgentResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return types.AgentResult{}, err
	}
	requestID := o.requestID()
	defer func() {
		if recovered := recover(); recovered != nil {
			o.record(EventRuntimeError, requestID, conversation, types.Metadata{"stage": "panic", "error": fmt.Sprint(recovered)})
			result = types.AgentResult{}
			err = core.WrapError(core.ErrProviderFailure, fmt.Sprintf("runtime panic: %v", recovered))
		}
	}()
	o.record(EventRequestStarted, requestID, conversation, nil)

	machine := NewStateMachine()
	_ = machine.Transition(StateRouting)
	o.record(EventStateChanged, requestID, conversation, types.Metadata{"state": string(machine.State())})

	intent, err := o.router.Route(ctx, conversation)
	if err != nil {
		o.record(EventRuntimeError, requestID, conversation, types.Metadata{"stage": "routing", "error": err.Error()})
		intent = fallbackIntent(conversation, "router_error")
	} else if intent.Confidence.Valid() && intent.Confidence < types.Confidence(0.5) {
		intent = fallbackIntent(conversation, "low_confidence")
	}
	o.record(EventRouteSelected, requestID, conversation, types.Metadata{"intent": string(intent.Name)})

	agent, err := o.agents.Find(intent)
	if err != nil {
		o.record(EventRuntimeError, requestID, conversation, types.Metadata{"stage": "agent_lookup", "error": err.Error()})
		result := safeResult("agent_not_found", types.Metadata{"intent": string(intent.Name)})
		o.record(EventDone, requestID, conversation, types.Metadata{"fallback": "agent_not_found"})
		return result, nil
	}
	o.record(EventAgentSelected, requestID, conversation, types.Metadata{"agent": agent.Name()})

	_ = machine.Transition(StateAgentRunning)
	o.record(EventStateChanged, requestID, conversation, types.Metadata{"state": string(machine.State())})

	result, err = agent.Run(ctx, conversation, intent)
	if err != nil {
		o.record(EventRuntimeError, requestID, conversation, types.Metadata{"stage": "agent_run", "agent": agent.Name(), "error": err.Error()})
		result := safeResult("agent_failed", types.Metadata{"agent": agent.Name()})
		o.record(EventDone, requestID, conversation, types.Metadata{"fallback": "agent_failed"})
		return result, nil
	}
	o.record(EventMessageCompleted, requestID, conversation, types.Metadata{"agent": result.AgentName})
	o.record(EventDone, requestID, conversation, nil)
	return result, nil
}

func (o Orchestrator) record(name EventName, requestID string, conversation types.Conversation, metadata types.Metadata) {
	o.recorder.Record(NewRuntimeEvent(name, requestID, conversation, metadata))
}

func (o Orchestrator) Stream(ctx context.Context, req types.TurnRequest, sink core.StreamSink) (types.AgentResult, error) {
	if o.coordinator == nil {
		return types.AgentResult{}, core.WrapError(core.ErrInvalidConfig, "streaming coordinator unavailable")
	}
	if sink == nil {
		return types.AgentResult{}, core.WrapError(core.ErrInvalidInput, "stream sink required")
	}
	if err := req.Validate(); err != nil {
		return types.AgentResult{}, core.WrapError(core.ErrInvalidInput, err.Error())
	}
	if err := ctx.Err(); err != nil {
		return types.AgentResult{}, err
	}

	requestID := o.requestID()
	emitter := streamEmitter{
		sink: sink,
		base: types.StreamEvent{
			RequestID:      requestID,
			TenantID:       req.TenantID,
			UserID:         req.UserID,
			ConversationID: req.ConversationID,
			TurnID:         req.TurnID,
			AttemptID:      req.AttemptID,
		},
	}
	if err := emitter.emit(ctx, types.StreamEventRequestStarted, nil, nil); err != nil {
		return types.AgentResult{}, err
	}

	session, err := o.coordinator.Begin(ctx, req, requestID)
	if err != nil {
		return types.AgentResult{}, err
	}
	if session.Replayed && session.ReplayResult != nil {
		if err := emitter.emit(ctx, types.StreamEventMessageCompleted, types.Metadata{
			"content":    session.ReplayResult.Message.Content,
			"agent_name": session.ReplayResult.AgentName,
		}, types.Metadata{"replayed": true}); err != nil {
			return types.AgentResult{}, err
		}
		if err := emitter.emit(ctx, types.StreamEventDone, types.Metadata{"status": "completed"}, types.Metadata{"replayed": true}); err != nil {
			return types.AgentResult{}, err
		}
		return *session.ReplayResult, nil
	}

	intent, err := o.router.Route(ctx, session.Window)
	if err != nil {
		intent = fallbackIntent(session.Window, "router_error")
	} else if intent.Confidence.Valid() && intent.Confidence < types.Confidence(0.5) {
		intent = fallbackIntent(session.Window, "low_confidence")
	}
	if err := emitter.emit(ctx, types.StreamEventRouteSelected, types.Metadata{"intent": string(intent.Name)}, nil); err != nil {
		_ = o.coordinator.Fail(ctx, session, "sink_failed")
		return types.AgentResult{}, err
	}

	agent, err := o.agents.Find(intent)
	if err != nil {
		_ = o.coordinator.Fail(ctx, session, "agent_not_found")
		if emitErr := emitter.emit(ctx, types.StreamEventError, types.Metadata{"code": "agent_not_found"}, nil); emitErr != nil {
			return types.AgentResult{}, emitErr
		}
		if emitErr := emitter.emit(ctx, types.StreamEventDone, types.Metadata{"status": "failed"}, nil); emitErr != nil {
			return types.AgentResult{}, emitErr
		}
		return safeResult("agent_not_found", types.Metadata{"intent": string(intent.Name)}), nil
	}
	if err := emitter.emit(ctx, types.StreamEventAgentSelected, types.Metadata{"agent": agent.Name()}, nil); err != nil {
		_ = o.coordinator.Fail(ctx, session, "sink_failed")
		return types.AgentResult{}, err
	}

	var result types.AgentResult
	if streamingAgent, ok := agent.(core.StreamingAgent); ok {
		result, err = streamingAgent.Stream(ctx, session.Window, intent, assistantDeltaSink{ctx: ctx, emitter: &emitter})
	} else {
		result, err = agent.Run(ctx, session.Window, intent)
	}
	if err != nil {
		if errorsIsCanceled(err) {
			_ = o.coordinator.Cancel(ctx, session)
			if emitErr := emitter.emit(ctx, types.StreamEventCanceled, types.Metadata{"status": "canceled"}, nil); emitErr != nil {
				return types.AgentResult{}, emitErr
			}
			if emitErr := emitter.emit(ctx, types.StreamEventDone, types.Metadata{"status": "canceled"}, nil); emitErr != nil {
				return types.AgentResult{}, emitErr
			}
			return types.AgentResult{}, err
		}
		_ = o.coordinator.Fail(ctx, session, "agent_failed")
		if emitErr := emitter.emit(ctx, types.StreamEventError, types.Metadata{"code": "agent_failed"}, nil); emitErr != nil {
			return types.AgentResult{}, emitErr
		}
		if emitErr := emitter.emit(ctx, types.StreamEventDone, types.Metadata{"status": "failed"}, nil); emitErr != nil {
			return types.AgentResult{}, emitErr
		}
		return types.AgentResult{}, err
	}

	result = ensureAssistantMessageIdentity(result, session, requestID)
	if err := o.coordinator.Complete(ctx, session, result); err != nil {
		_ = o.coordinator.Fail(ctx, session, "commit_failed")
		return types.AgentResult{}, err
	}
	if err := emitter.emit(ctx, types.StreamEventMessageCompleted, types.Metadata{
		"content":    result.Message.Content,
		"agent_name": result.AgentName,
	}, nil); err != nil {
		return types.AgentResult{}, err
	}
	if err := emitter.emit(ctx, types.StreamEventDone, types.Metadata{"status": "completed"}, nil); err != nil {
		return types.AgentResult{}, err
	}
	return result, nil
}

func validateConversation(conversation types.Conversation) error {
	switch {
	case strings.TrimSpace(conversation.ID) == "":
		return core.NewNamedError(core.ErrInvalidInput, "conversation", "id")
	case strings.TrimSpace(conversation.TenantID) == "":
		return core.NewNamedError(core.ErrInvalidInput, "conversation", "tenant_id")
	case strings.TrimSpace(conversation.UserID) == "":
		return core.NewNamedError(core.ErrInvalidInput, "conversation", "user_id")
	case !hasUserMessage(conversation.Messages):
		return core.NewNamedError(core.ErrInvalidInput, "conversation", "user_message")
	default:
		return nil
	}
}

func hasUserMessage(messages []types.Message) bool {
	for _, message := range messages {
		if message.Role == types.RoleUser && strings.TrimSpace(message.Content) != "" {
			return true
		}
	}
	return false
}

func fallbackIntent(conversation types.Conversation, reason string) types.Intent {
	return types.Intent{
		Name:       types.IntentPersonaChat,
		Query:      lastUserMessage(conversation.Messages),
		Confidence: types.Confidence(0.3),
		Metadata:   types.Metadata{"fallback": reason},
	}
}

func lastUserMessage(messages []types.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == types.RoleUser {
			return messages[i].Content
		}
	}
	return ""
}

func safeResult(reason string, metadata types.Metadata) types.AgentResult {
	resultMetadata := copyMetadata(metadata)
	if resultMetadata == nil {
		resultMetadata = types.Metadata{}
	}
	resultMetadata["error"] = reason
	return types.AgentResult{
		AgentName: "runtime-fallback",
		Message: types.Message{
			Role:    types.RoleAssistant,
			Content: "I could not complete that request safely, but the runtime recovered.",
		},
		Confidence: types.Confidence(0.1),
		Metadata:   resultMetadata,
	}
}

var _ core.Orchestrator = Orchestrator{}
var _ core.StreamingOrchestrator = Orchestrator{}

type streamEmitter struct {
	sink     core.StreamSink
	base     types.StreamEvent
	sequence uint64
}

func (e *streamEmitter) emit(ctx context.Context, name types.StreamEventName, payload, metadata types.Metadata) error {
	e.sequence++
	event := e.base
	event.Name = name
	event.Sequence = e.sequence
	event.Timestamp = time.Now().UTC()
	event.Payload = copyStreamMetadata(payload)
	event.Metadata = copyStreamMetadata(metadata)
	return e.sink.Emit(ctx, event)
}

type assistantDeltaSink struct {
	ctx     context.Context
	emitter *streamEmitter
}

func (s assistantDeltaSink) EmitAssistantDelta(ctx context.Context, text string) error {
	if ctx == nil {
		ctx = s.ctx
	}
	return s.emitter.emit(ctx, types.StreamEventAssistantDelta, types.Metadata{"content": text}, nil)
}

func copyStreamMetadata(metadata types.Metadata) types.Metadata {
	if metadata == nil {
		return nil
	}
	copied := make(types.Metadata, len(metadata))
	for key, value := range metadata {
		copied[key] = value
	}
	return copied
}

func errorsIsCanceled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func ensureAssistantMessageIdentity(result types.AgentResult, session *conversation.TurnSession, requestID string) types.AgentResult {
	if strings.TrimSpace(result.Message.ID) != "" {
		return result
	}
	cloned := result
	cloned.Message.ID = fmt.Sprintf("assistant-%s-%s", session.TurnID, requestID)
	if cloned.Message.Role == "" {
		cloned.Message.Role = types.RoleAssistant
	}
	if cloned.Message.CreatedAt.IsZero() {
		cloned.Message.CreatedAt = time.Now().UTC()
	}
	return cloned
}

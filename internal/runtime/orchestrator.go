package runtime

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/pkg/types"
)

var requestCounter atomic.Uint64

type OrchestratorConfig struct {
	Router    core.Router
	Agents    *core.AgentRegistry
	Recorder  *EventRecorder
	RequestID func() string
}

type Orchestrator struct {
	router    core.Router
	agents    *core.AgentRegistry
	recorder  *EventRecorder
	requestID func() string
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

package core

import (
	"context"

	"github.com/nobodycan/digital-twin/pkg/types"
)

// Agent handles a routed intent within a conversation.
type Agent interface {
	Name() string
	CanHandle(types.Intent) bool
	Run(context.Context, types.Conversation, types.Intent) (types.AgentResult, error)
}

// Skill executes a named tool-like capability with structured arguments.
type Skill interface {
	Name() string
	Run(context.Context, map[string]any) (types.SkillResult, error)
}

// Router classifies a conversation into an intent.
type Router interface {
	Route(context.Context, types.Conversation) (types.Intent, error)
}

// Orchestrator coordinates routing and agent execution for a conversation.
type Orchestrator interface {
	Handle(context.Context, types.Conversation) (types.AgentResult, error)
}

// StreamSink emits typed streaming events.
type StreamSink interface {
	Emit(context.Context, types.StreamEvent) error
}

// AssistantDeltaSink emits accepted assistant text segments.
type AssistantDeltaSink interface {
	EmitAssistantDelta(context.Context, string) error
}

// StreamingAgent adds incremental output to an Agent.
type StreamingAgent interface {
	Agent
	Stream(context.Context, types.Conversation, types.Intent, AssistantDeltaSink) (types.AgentResult, error)
}

// StreamingOrchestrator adds typed streaming to an Orchestrator.
type StreamingOrchestrator interface {
	Orchestrator
	Stream(context.Context, types.TurnRequest, StreamSink) (types.AgentResult, error)
}

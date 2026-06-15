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

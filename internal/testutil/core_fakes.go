package testutil

import (
	"context"
	"sync"

	"github.com/nobodycan/digital-twin/pkg/types"
)

// AgentCall records a FakeAgent invocation.
type AgentCall struct {
	Conversation types.Conversation
	Intent       types.Intent
}

// FakeAgent is a deterministic core.Agent implementation for tests.
type FakeAgent struct {
	AgentName string
	Result    types.AgentResult
	Err       error
	Handles   bool

	mu    sync.Mutex
	calls []AgentCall
}

// Name returns the configured fake agent name.
func (f *FakeAgent) Name() string {
	return f.AgentName
}

// CanHandle returns the configured Handles value.
func (f *FakeAgent) CanHandle(types.Intent) bool {
	return f.Handles
}

// Run records the call and returns the configured result or error.
func (f *FakeAgent) Run(_ context.Context, conversation types.Conversation, intent types.Intent) (types.AgentResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, AgentCall{Conversation: conversation, Intent: intent})
	f.mu.Unlock()
	return f.Result, f.Err
}

// Calls returns a copy of recorded calls.
func (f *FakeAgent) Calls() []AgentCall {
	f.mu.Lock()
	defer f.mu.Unlock()

	calls := make([]AgentCall, len(f.calls))
	copy(calls, f.calls)
	return calls
}

// FakeSkill is a deterministic core.Skill implementation for tests.
type FakeSkill struct {
	SkillName string
	Result    types.SkillResult
	Err       error
}

// Name returns the configured fake skill name.
func (f *FakeSkill) Name() string {
	return f.SkillName
}

// Run returns the configured result or error.
func (f *FakeSkill) Run(context.Context, map[string]any) (types.SkillResult, error) {
	return f.Result, f.Err
}

// FakeRouter is a deterministic core.Router implementation for tests.
type FakeRouter struct {
	Intent types.Intent
	Err    error
}

// Route returns the configured intent or error.
func (f *FakeRouter) Route(context.Context, types.Conversation) (types.Intent, error) {
	return f.Intent, f.Err
}

// FakeOrchestrator is a deterministic core.Orchestrator implementation for tests.
type FakeOrchestrator struct {
	Result types.AgentResult
	Err    error
}

// Handle returns the configured result or error.
func (f *FakeOrchestrator) Handle(context.Context, types.Conversation) (types.AgentResult, error) {
	return f.Result, f.Err
}

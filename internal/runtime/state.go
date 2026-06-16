package runtime

import (
	"errors"
	"fmt"
)

type State string

const (
	StateReceived     State = "received"
	StateRouting      State = "routing"
	StateAgentRunning State = "agent_running"
	StateCompleted    State = "completed"
	StateFailed       State = "failed"
	StateFallback     State = "fallback"
)

var ErrInvalidTransition = errors.New("invalid transition")

type StateMachine struct {
	state State
}

func NewStateMachine() *StateMachine {
	return &StateMachine{state: StateReceived}
}

func (m *StateMachine) State() State {
	return m.state
}

func (m *StateMachine) Transition(next State) error {
	if !canTransition(m.state, next) {
		return fmt.Errorf("%s -> %s: %w", m.state, next, ErrInvalidTransition)
	}
	m.state = next
	return nil
}

func canTransition(from, to State) bool {
	switch from {
	case StateReceived:
		return to == StateRouting || to == StateFailed
	case StateRouting:
		return to == StateAgentRunning || to == StateFallback || to == StateFailed
	case StateAgentRunning:
		return to == StateCompleted || to == StateFallback || to == StateFailed
	default:
		return false
	}
}

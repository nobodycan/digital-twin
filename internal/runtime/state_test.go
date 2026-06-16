package runtime

import (
	"errors"
	"testing"
)

func TestStateMachineAllowsValidRuntimeTransitions(t *testing.T) {
	machine := NewStateMachine()

	for _, next := range []State{StateRouting, StateAgentRunning, StateCompleted} {
		if err := machine.Transition(next); err != nil {
			t.Fatalf("Transition(%s) error = %v", next, err)
		}
	}

	if machine.State() != StateCompleted {
		t.Fatalf("State() = %s, want completed", machine.State())
	}
}

func TestStateMachineRejectsInvalidTransition(t *testing.T) {
	machine := NewStateMachine()

	err := machine.Transition(StateCompleted)
	if err == nil {
		t.Fatal("Transition(completed) error = nil, want invalid transition")
	}
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("Transition(completed) error = %v, want ErrInvalidTransition", err)
	}
	if machine.State() != StateReceived {
		t.Fatalf("State() = %s, want received", machine.State())
	}
}

func TestStateMachineTerminalStateCannotTransition(t *testing.T) {
	machine := NewStateMachine()
	if err := machine.Transition(StateRouting); err != nil {
		t.Fatalf("Transition(routing): %v", err)
	}
	if err := machine.Transition(StateFallback); err != nil {
		t.Fatalf("Transition(fallback): %v", err)
	}

	err := machine.Transition(StateCompleted)
	if err == nil {
		t.Fatal("Transition from fallback error = nil, want invalid transition")
	}
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("Transition from fallback error = %v, want ErrInvalidTransition", err)
	}
}

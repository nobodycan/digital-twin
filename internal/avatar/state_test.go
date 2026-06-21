package avatar

import "testing"

func TestStateMachineMapsConversationSignalsToSupportedStates(t *testing.T) {
	machine, err := NewStateMachine(Manifest{
		Supported:     []State{StateIdle, StateListening, StateThinking, StateSpeaking, StateError, StateInterrupted},
		FallbackState: StateIdle,
	})
	if err != nil {
		t.Fatalf("NewStateMachine returned error: %v", err)
	}

	tests := []struct {
		signal Signal
		want   State
	}{
		{SignalUserInput, StateListening},
		{SignalRuntimeThinking, StateThinking},
		{SignalAssistantSpeaking, StateSpeaking},
		{SignalRuntimeError, StateError},
		{SignalInterrupted, StateInterrupted},
		{SignalDone, StateIdle},
	}

	for _, test := range tests {
		if got := machine.Next(test.signal); got != test.want {
			t.Fatalf("Next(%q) = %q, want %q", test.signal, got, test.want)
		}
	}
}

func TestStateMachineFallsBackWhenMappedStateIsUnsupported(t *testing.T) {
	machine, err := NewStateMachine(Manifest{
		Supported:     []State{StateIdle},
		FallbackState: StateIdle,
	})
	if err != nil {
		t.Fatalf("NewStateMachine returned error: %v", err)
	}

	if got := machine.Next(SignalAssistantSpeaking); got != StateIdle {
		t.Fatalf("unsupported speaking state should fall back to idle, got %q", got)
	}
}

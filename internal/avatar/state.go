package avatar

type Signal string

const (
	SignalUserInput         Signal = "user_input"
	SignalRuntimeThinking   Signal = "runtime_thinking"
	SignalAssistantSpeaking Signal = "assistant_speaking"
	SignalRuntimeError      Signal = "runtime_error"
	SignalInterrupted       Signal = "interrupted"
	SignalDone              Signal = "done"
)

type StateMachine struct {
	fallback  State
	supported map[State]struct{}
}

func NewStateMachine(manifest Manifest) (StateMachine, error) {
	if len(manifest.Supported) == 0 || manifest.FallbackState == "" {
		return StateMachine{}, ErrInvalidManifest
	}
	machine := StateMachine{
		fallback:  manifest.FallbackState,
		supported: make(map[State]struct{}, len(manifest.Supported)),
	}
	for _, state := range manifest.Supported {
		machine.supported[state] = struct{}{}
	}
	if !machine.supports(machine.fallback) {
		return StateMachine{}, ErrInvalidManifest
	}
	return machine, nil
}

func (m StateMachine) Next(signal Signal) State {
	switch signal {
	case SignalUserInput:
		return m.withFallback(StateListening)
	case SignalRuntimeThinking:
		return m.withFallback(StateThinking)
	case SignalAssistantSpeaking:
		return m.withFallback(StateSpeaking)
	case SignalRuntimeError:
		return m.withFallback(StateError)
	case SignalInterrupted:
		return m.withFallback(StateInterrupted)
	case SignalDone:
		return m.withFallback(StateIdle)
	default:
		return m.fallback
	}
}

func (m StateMachine) withFallback(state State) State {
	if m.supports(state) {
		return state
	}
	return m.fallback
}

func (m StateMachine) supports(state State) bool {
	_, ok := m.supported[state]
	return ok
}

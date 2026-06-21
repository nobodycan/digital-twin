package presentation

import (
	"context"
	"time"

	"github.com/nobodycan/digital-twin/internal/avatar"
	"github.com/nobodycan/digital-twin/internal/voice"
	"github.com/nobodycan/digital-twin/pkg/types"
)

type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now().UTC()
}

type Adapter struct {
	TTS    voice.TTSClient
	Avatar avatar.StateMachine
	Clock  Clock
}

type AdaptRequest struct {
	Context EventContext
	Result  types.AgentResult
}

func (a Adapter) Adapt(request AdaptRequest) ([]Event, error) {
	clock := a.Clock
	if clock == nil {
		clock = systemClock{}
	}
	context := request.Context
	context.OccurredAt = clock.Now()

	resultText := request.Result.Message.Content
	events := []Event{
		NewEvent(EventConversationStarted, withSequence(context, 1), nil, nil),
		NewEvent(EventAssistantTextDelta, withSequence(context, 2), map[string]any{
			"text":       resultText,
			"agent_name": request.Result.AgentName,
			"confidence": request.Result.Confidence,
		}, request.Result.Metadata),
	}

	timeline, err := BuildSpeechTimeline(resultText, TimelineOptions{StartAt: context.OccurredAt})
	if err != nil {
		return nil, err
	}
	events = append(events, NewEvent(EventSubtitle, withSequence(context, 3), map[string]any{
		"subtitles": timeline.Subtitles,
		"visemes":   timeline.Visemes,
	}, nil))

	if a.TTS != nil {
		audio, err := a.TTS.Synthesize(contextOrBackground(context), voice.TTSRequest{Text: resultText})
		if err != nil {
			events = append(events, NewErrorEvent(withSequence(context, 4), "tts unavailable", err.Error(), "continue with text or retry voice"))
		} else {
			events = append(events, NewEvent(EventAudioChunk, withSequence(context, 4), map[string]any{
				"provider": audio.Provider,
				"chunks":   audio.Chunks,
			}, nil))
		}
	}

	events = append(events, NewEvent(EventAvatarState, withSequence(context, 5), map[string]any{
		"state": string(a.Avatar.Next(avatar.SignalAssistantSpeaking)),
	}, nil))
	events = append(events, NewEvent(EventDone, withSequence(context, 6), map[string]any{"status": "ok"}, nil))

	return events, nil
}

func withSequence(context EventContext, sequence int) EventContext {
	context.Sequence = sequence
	return context
}

func contextOrBackground(EventContext) context.Context {
	return context.Background()
}

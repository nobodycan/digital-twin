package presentation

import (
	"context"
	"fmt"
	"time"

	"github.com/nobodycan/digital-twin/internal/avatar"
	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/internal/voice"
	"github.com/nobodycan/digital-twin/pkg/types"
)

type PresentationSink interface {
	Emit(context.Context, Event) error
}

type AdapterStreamSink struct {
	adapter         Adapter
	sink            PresentationSink
	sequence        int
	accumulatedText string
	spoke           bool
}

func (a Adapter) NewStreamSink(sink PresentationSink) *AdapterStreamSink {
	return &AdapterStreamSink{
		adapter: a,
		sink:    sink,
	}
}

func (s *AdapterStreamSink) Emit(ctx context.Context, event types.StreamEvent) error {
	switch event.Name {
	case types.StreamEventRequestStarted:
		if err := s.emit(ctx, eventContextFromStream(event), EventConversationStarted, nil, nil); err != nil {
			return err
		}
		return s.emit(ctx, eventContextFromStream(event), EventAvatarState, map[string]any{
			"state": string(s.adapter.Avatar.Next(avatar.SignalRuntimeThinking)),
		}, nil)
	case types.StreamEventAssistantDelta:
		text, _ := event.Payload["content"].(string)
		s.accumulatedText += text
		if err := s.emit(ctx, eventContextFromStream(event), EventAssistantTextDelta, map[string]any{
			"text": text,
		}, nil); err != nil {
			return err
		}
		if !s.spoke {
			s.spoke = true
			return s.emit(ctx, eventContextFromStream(event), EventAvatarState, map[string]any{
				"state": string(s.adapter.Avatar.Next(avatar.SignalAssistantSpeaking)),
			}, nil)
		}
		return nil
	case types.StreamEventMessageCompleted:
		return s.emitCompletion(ctx, event)
	case types.StreamEventCanceled:
		if err := s.emit(ctx, eventContextFromStream(event), EventInterrupted, event.Payload, nil); err != nil {
			return err
		}
		return s.emit(ctx, eventContextFromStream(event), EventAvatarState, map[string]any{
			"state": string(s.adapter.Avatar.Next(avatar.SignalInterrupted)),
		}, nil)
	case types.StreamEventError:
		if err := s.emit(ctx, eventContextFromStream(event), EventError, map[string]any{
			"problem": payloadString(event.Payload, "code", "stream_error"),
			"cause":   payloadString(event.Payload, "cause", ""),
			"fix":     "retry",
		}, nil); err != nil {
			return err
		}
		return s.emit(ctx, eventContextFromStream(event), EventAvatarState, map[string]any{
			"state": string(s.adapter.Avatar.Next(avatar.SignalRuntimeError)),
		}, nil)
	case types.StreamEventDone:
		if err := s.emit(ctx, eventContextFromStream(event), EventDone, event.Payload, nil); err != nil {
			return err
		}
		status := payloadString(event.Payload, "status", "")
		if status == "completed" {
			return s.emit(ctx, eventContextFromStream(event), EventAvatarState, map[string]any{
				"state": string(s.adapter.Avatar.Next(avatar.SignalDone)),
			}, nil)
		}
		return nil
	default:
		return nil
	}
}

func (s *AdapterStreamSink) emitCompletion(ctx context.Context, event types.StreamEvent) error {
	text := payloadString(event.Payload, "content", s.accumulatedText)
	if text == "" {
		text = s.accumulatedText
	}
	timeline, err := BuildSpeechTimeline(text, TimelineOptions{StartAt: event.Timestamp})
	if err != nil {
		return err
	}
	if err := s.emit(ctx, eventContextFromStream(event), EventSubtitle, map[string]any{
		"subtitles": timeline.Subtitles,
		"visemes":   timeline.Visemes,
	}, nil); err != nil {
		return err
	}

	if s.adapter.TTS != nil {
		audio, err := s.adapter.TTS.Synthesize(ctx, voice.TTSRequest{Text: text})
		if err != nil {
			return s.emit(ctx, eventContextFromStream(event), EventError, map[string]any{
				"problem": "tts unavailable",
				"cause":   err.Error(),
				"fix":     "continue with text or retry voice",
			}, nil)
		}
		if err := s.emit(ctx, eventContextFromStream(event), EventAudioChunk, map[string]any{
			"provider": audio.Provider,
			"chunks":   audio.Chunks,
		}, nil); err != nil {
			return err
		}
	}
	return nil
}

func (s *AdapterStreamSink) emit(ctx context.Context, eventContext EventContext, name EventName, payload, metadata map[string]any) error {
	s.sequence++
	eventContext.Sequence = s.sequence
	if eventContext.OccurredAt.IsZero() {
		eventContext.OccurredAt = s.now()
	}
	return s.sink.Emit(ctx, NewEvent(name, eventContext, payload, metadata))
}

func (s *AdapterStreamSink) now() time.Time {
	if s.adapter.Clock != nil {
		return s.adapter.Clock.Now()
	}
	return time.Now().UTC()
}

func eventContextFromStream(event types.StreamEvent) EventContext {
	return EventContext{
		TenantID:       event.TenantID,
		UserID:         event.UserID,
		ConversationID: event.ConversationID,
		RequestID:      event.RequestID,
		OccurredAt:     event.Timestamp,
	}
}

func payloadString(payload types.Metadata, key, fallback string) string {
	if payload == nil {
		return fallback
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return fallback
	}
	return fmt.Sprint(value)
}

var _ core.StreamSink = (*AdapterStreamSink)(nil)

package voice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrEmptyInput = errors.New("voice input is empty")

type TTSClient interface {
	Synthesize(context.Context, TTSRequest) (TTSResult, error)
}

type ASRClient interface {
	Transcribe(context.Context, ASRRequest) (ASRResult, error)
}

type TTSRequest struct {
	Text    string  `json:"text"`
	Voice   string  `json:"voice,omitempty"`
	Speed   float64 `json:"speed,omitempty"`
	Emotion string  `json:"emotion,omitempty"`
}

type TTSResult struct {
	Provider string       `json:"provider"`
	Chunks   []AudioChunk `json:"chunks"`
}

type AudioChunk struct {
	Index    int           `json:"index"`
	MimeType string        `json:"mime_type"`
	Data     []byte        `json:"data"`
	Start    time.Duration `json:"start"`
	Duration time.Duration `json:"duration"`
}

type ASRRequest struct {
	Audio      []byte `json:"audio"`
	Language   string `json:"language,omitempty"`
	SampleRate int    `json:"sample_rate,omitempty"`
}

type ASRResult struct {
	Provider string              `json:"provider"`
	Text     string              `json:"text"`
	Segments []TranscriptSegment `json:"segments"`
}

type TranscriptSegment struct {
	Text       string        `json:"text"`
	Start      time.Duration `json:"start"`
	End        time.Duration `json:"end"`
	Confidence float64       `json:"confidence"`
}

type MockTTSClient struct{}

func (MockTTSClient) Synthesize(_ context.Context, request TTSRequest) (TTSResult, error) {
	words := strings.Fields(request.Text)
	if len(words) == 0 {
		return TTSResult{}, ErrEmptyInput
	}
	voice := valueOrDefault(request.Voice, "local")
	emotion := valueOrDefault(request.Emotion, "neutral")
	chunks := make([]AudioChunk, 0, len(words))
	for index, word := range words {
		chunks = append(chunks, AudioChunk{
			Index:    index,
			MimeType: "audio/mock",
			Data:     []byte(fmt.Sprintf("mock-audio:%s:%s:%d:%s", voice, emotion, index, word)),
			Start:    time.Duration(index) * 300 * time.Millisecond,
			Duration: 300 * time.Millisecond,
		})
	}
	return TTSResult{Provider: "mock", Chunks: chunks}, nil
}

type MockASRClient struct{}

func (MockASRClient) Transcribe(_ context.Context, request ASRRequest) (ASRResult, error) {
	text := strings.TrimSpace(string(request.Audio))
	words := strings.Fields(text)
	if len(words) == 0 {
		return ASRResult{}, ErrEmptyInput
	}
	segments := make([]TranscriptSegment, 0, len(words))
	for index, word := range words {
		start := time.Duration(index) * 300 * time.Millisecond
		segments = append(segments, TranscriptSegment{
			Text:       word,
			Start:      start,
			End:        start + 300*time.Millisecond,
			Confidence: 1,
		})
	}
	return ASRResult{Provider: "mock", Text: text, Segments: segments}, nil
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

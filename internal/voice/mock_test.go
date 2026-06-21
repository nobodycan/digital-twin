package voice

import (
	"context"
	"testing"
	"time"
)

func TestMockTTSClientReturnsDeterministicAudioChunks(t *testing.T) {
	client := MockTTSClient{}

	result, err := client.Synthesize(context.Background(), TTSRequest{
		Text:    "Hello digital twin",
		Voice:   "local-professional",
		Speed:   1.0,
		Emotion: "calm",
	})
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}

	if result.Provider != "mock" {
		t.Fatalf("provider = %q, want mock", result.Provider)
	}
	if len(result.Chunks) != 3 {
		t.Fatalf("chunk count = %d, want 3", len(result.Chunks))
	}
	if string(result.Chunks[0].Data) != "mock-audio:local-professional:calm:0:Hello" {
		t.Fatalf("first chunk data = %q", string(result.Chunks[0].Data))
	}
	if result.Chunks[1].Start <= result.Chunks[0].Start {
		t.Fatalf("chunks should have increasing start offsets")
	}
}

func TestMockASRClientReturnsDeterministicTranscriptSegments(t *testing.T) {
	client := MockASRClient{}

	result, err := client.Transcribe(context.Background(), ASRRequest{
		Audio:      []byte("hello from audio"),
		Language:   "en-US",
		SampleRate: 16000,
	})
	if err != nil {
		t.Fatalf("Transcribe returned error: %v", err)
	}

	if result.Provider != "mock" {
		t.Fatalf("provider = %q, want mock", result.Provider)
	}
	if result.Text != "hello from audio" {
		t.Fatalf("text = %q, want transcript from audio bytes", result.Text)
	}
	if len(result.Segments) != 3 {
		t.Fatalf("segment count = %d, want 3", len(result.Segments))
	}
	if result.Segments[0].Confidence != 1 {
		t.Fatalf("confidence = %v, want 1", result.Segments[0].Confidence)
	}
	if result.Segments[0].End != 300*time.Millisecond {
		t.Fatalf("first segment end = %s, want 300ms", result.Segments[0].End)
	}
}

func TestMockVoiceClientsRejectEmptyInput(t *testing.T) {
	if _, err := (MockTTSClient{}).Synthesize(context.Background(), TTSRequest{}); err == nil {
		t.Fatalf("expected empty TTS text to be rejected")
	}
	if _, err := (MockASRClient{}).Transcribe(context.Background(), ASRRequest{}); err == nil {
		t.Fatalf("expected empty ASR audio to be rejected")
	}
}

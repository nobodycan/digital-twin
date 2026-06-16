package skills

import (
	"context"
	"errors"
	"testing"
)

func TestPresentationSkillsReturnDeterministicPlaceholders(t *testing.T) {
	tts, err := NewTTSSpeakSkill().Run(context.Background(), map[string]any{"text": "hello"})
	if err != nil {
		t.Fatalf("tts_speak Run() error = %v", err)
	}
	if tts.Metadata["placeholder"] != true {
		t.Fatalf("tts metadata = %#v, want placeholder", tts.Metadata)
	}

	asr, err := NewASRTranscribeSkill().Run(context.Background(), map[string]any{"audio_ref": "audio.wav"})
	if err != nil {
		t.Fatalf("asr_transcribe Run() error = %v", err)
	}
	if asr.Output != "" {
		t.Fatalf("asr output = %v, want empty deterministic transcript", asr.Output)
	}

	avatar, err := NewAvatarStateSkill().Run(context.Background(), map[string]any{"state": "thinking"})
	if err != nil {
		t.Fatalf("avatar_state Run() error = %v", err)
	}
	if avatar.Output != "thinking" {
		t.Fatalf("avatar output = %v, want thinking", avatar.Output)
	}

	subtitle, err := NewSubtitleTimelineSkill().Run(context.Background(), map[string]any{"text": "hello world"})
	if err != nil {
		t.Fatalf("subtitle_timeline Run() error = %v", err)
	}
	if subtitle.SkillName != "subtitle_timeline" {
		t.Fatalf("subtitle skill = %q", subtitle.SkillName)
	}
}

func TestPresentationSkillsRejectInvalidParams(t *testing.T) {
	_, err := NewTTSSpeakSkill().Run(context.Background(), map[string]any{"text": 123})
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("tts error = %v, want ErrInvalidParams", err)
	}
}

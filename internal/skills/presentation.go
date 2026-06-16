package skills

import (
	"context"
	"strings"

	"github.com/nobodycan/digital-twin/pkg/types"
)

type TTSSpeakSkill struct{}

func NewTTSSpeakSkill() TTSSpeakSkill { return TTSSpeakSkill{} }
func (s TTSSpeakSkill) Name() string  { return "tts_speak" }

func (s TTSSpeakSkill) Run(_ context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{{Name: "text", Type: String, Required: true}}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	return types.SkillResult{SkillName: s.Name(), Output: "tts://" + valid["text"].(string), Metadata: types.Metadata{"placeholder": true}}, nil
}

type ASRTranscribeSkill struct{}

func NewASRTranscribeSkill() ASRTranscribeSkill { return ASRTranscribeSkill{} }
func (s ASRTranscribeSkill) Name() string       { return "asr_transcribe" }

func (s ASRTranscribeSkill) Run(_ context.Context, params map[string]any) (types.SkillResult, error) {
	_, err := (Spec{Params: []Param{{Name: "audio_ref", Type: String, Required: true}}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	return types.SkillResult{SkillName: s.Name(), Output: "", Metadata: types.Metadata{"placeholder": true}}, nil
}

type AvatarStateSkill struct{}

func NewAvatarStateSkill() AvatarStateSkill { return AvatarStateSkill{} }
func (s AvatarStateSkill) Name() string     { return "avatar_state" }

func (s AvatarStateSkill) Run(_ context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{{Name: "state", Type: String, Required: true}}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	return types.SkillResult{SkillName: s.Name(), Output: valid["state"], Metadata: types.Metadata{"placeholder": true}}, nil
}

type SubtitleTimelineSkill struct{}

func NewSubtitleTimelineSkill() SubtitleTimelineSkill { return SubtitleTimelineSkill{} }
func (s SubtitleTimelineSkill) Name() string          { return "subtitle_timeline" }

func (s SubtitleTimelineSkill) Run(_ context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{{Name: "text", Type: String, Required: true}}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	words := strings.Fields(valid["text"].(string))
	return types.SkillResult{SkillName: s.Name(), Output: words, Metadata: types.Metadata{"placeholder": true}}, nil
}

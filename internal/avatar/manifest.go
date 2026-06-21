package avatar

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
)

var ErrInvalidManifest = errors.New("invalid avatar manifest")

type Type string

const (
	Type2D        Type = "2d"
	TypeLive2D    Type = "live2d"
	Type3D        Type = "3d"
	TypeVideo     Type = "video"
	TypeVoiceOnly Type = "voice_only"
)

type State string

const (
	StateIdle        State = "idle"
	StateListening   State = "listening"
	StateThinking    State = "thinking"
	StateSpeaking    State = "speaking"
	StateHappy       State = "happy"
	StateApologetic  State = "apologetic"
	StateSerious     State = "serious"
	StateConfused    State = "confused"
	StateInterrupted State = "interrupted"
	StateError       State = "error"
)

type Manifest struct {
	ID            string       `json:"id"`
	DisplayName   string       `json:"display_name"`
	Version       string       `json:"version"`
	Type          Type         `json:"type"`
	AssetURI      string       `json:"asset_uri"`
	AssetHash     string       `json:"asset_hash"`
	License       License      `json:"license"`
	Supported     []State      `json:"supported_states"`
	FallbackState State        `json:"fallback_state"`
	Capabilities  Capabilities `json:"capabilities,omitempty"`
}

type License struct {
	Name        string `json:"name"`
	Attribution string `json:"attribution"`
}

type Capabilities struct {
	Expressions bool `json:"expressions"`
	Visemes     bool `json:"visemes"`
}

func (m Manifest) Validate() error {
	if strings.TrimSpace(m.ID) == "" ||
		strings.TrimSpace(m.DisplayName) == "" ||
		strings.TrimSpace(m.Version) == "" ||
		!m.Type.Valid() ||
		strings.TrimSpace(m.AssetURI) == "" ||
		strings.TrimSpace(m.AssetHash) == "" ||
		strings.TrimSpace(m.License.Name) == "" ||
		strings.TrimSpace(m.License.Attribution) == "" ||
		len(m.Supported) == 0 ||
		strings.TrimSpace(string(m.FallbackState)) == "" {
		return ErrInvalidManifest
	}
	if !m.supports(m.FallbackState) {
		return ErrInvalidManifest
	}
	return nil
}

func LoadManifestFile(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func (t Type) Valid() bool {
	switch t {
	case Type2D, TypeLive2D, Type3D, TypeVideo, TypeVoiceOnly:
		return true
	default:
		return false
	}
}

func (m Manifest) supports(state State) bool {
	for _, supported := range m.Supported {
		if supported == state {
			return true
		}
	}
	return false
}

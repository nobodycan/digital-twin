package avatar

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestValidateAcceptsCompleteLocalAvatar(t *testing.T) {
	manifest := Manifest{
		ID:            "avatar-local-1",
		DisplayName:   "Local Professional Twin",
		Version:       "0.1.0",
		Type:          Type2D,
		AssetURI:      "assets/avatar/local/avatar.json",
		AssetHash:     "sha256:abc123",
		License:       License{Name: "local-fixture", Attribution: "digital-twin"},
		Supported:     []State{StateIdle, StateListening, StateThinking, StateSpeaking},
		FallbackState: StateIdle,
		Capabilities:  Capabilities{Expressions: true, Visemes: true},
	}

	if err := manifest.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestManifestValidateRejectsMissingFallbackState(t *testing.T) {
	manifest := Manifest{
		ID:          "avatar-local-1",
		DisplayName: "Local Professional Twin",
		Version:     "0.1.0",
		Type:        Type2D,
		AssetURI:    "assets/avatar/local/avatar.json",
		AssetHash:   "sha256:abc123",
		License:     License{Name: "local-fixture", Attribution: "digital-twin"},
		Supported:   []State{StateIdle, StateListening},
	}

	if err := manifest.Validate(); err == nil {
		t.Fatalf("expected missing fallback state to be rejected")
	}
}

func TestManifestValidateRejectsFallbackNotInSupportedStates(t *testing.T) {
	manifest := Manifest{
		ID:            "avatar-local-1",
		DisplayName:   "Local Professional Twin",
		Version:       "0.1.0",
		Type:          Type2D,
		AssetURI:      "assets/avatar/local/avatar.json",
		AssetHash:     "sha256:abc123",
		License:       License{Name: "local-fixture", Attribution: "digital-twin"},
		Supported:     []State{StateListening},
		FallbackState: StateIdle,
	}

	if err := manifest.Validate(); err == nil {
		t.Fatalf("expected unsupported fallback state to be rejected")
	}
}

func TestLoadManifestFileValidatesSampleFixture(t *testing.T) {
	manifest, err := LoadManifestFile("testdata/local_avatar.json")
	if err != nil {
		t.Fatalf("LoadManifestFile returned error: %v", err)
	}

	if manifest.ID != "local-professional-twin" {
		t.Fatalf("manifest ID = %q", manifest.ID)
	}
	if manifest.FallbackState != StateIdle {
		t.Fatalf("fallback state = %q, want %q", manifest.FallbackState, StateIdle)
	}
}

func TestSampleManifestAssetReferenceExists(t *testing.T) {
	manifest, err := LoadManifestFile("testdata/local_avatar.json")
	if err != nil {
		t.Fatalf("LoadManifestFile returned error: %v", err)
	}

	assetPath := filepath.Join("..", "..", filepath.FromSlash(manifest.AssetURI))
	if _, err := os.Stat(assetPath); err != nil {
		t.Fatalf("sample asset reference %q should exist: %v", manifest.AssetURI, err)
	}
}

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFromYAML(t *testing.T) {
	path := writeConfig(t, `
server:
  port: 9090
log:
  level: debug
llm:
  api_key: yaml-key
db:
  dsn: postgres://user:pass@localhost:5432/twin
tts:
  provider: azure
asr:
  provider: whisper
object_storage:
  endpoint: http://minio:9000
tenant:
  default_id: tenant-a
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Fatalf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Log.Level != "debug" {
		t.Fatalf("Log.Level = %q, want debug", cfg.Log.Level)
	}
	if cfg.LLM.APIKey != "yaml-key" {
		t.Fatalf("LLM.APIKey = %q, want yaml-key", cfg.LLM.APIKey)
	}
	if cfg.DB.DSN != "postgres://user:pass@localhost:5432/twin" {
		t.Fatalf("DB.DSN = %q", cfg.DB.DSN)
	}
	if cfg.TTS.Provider != "azure" {
		t.Fatalf("TTS.Provider = %q, want azure", cfg.TTS.Provider)
	}
	if cfg.ASR.Provider != "whisper" {
		t.Fatalf("ASR.Provider = %q, want whisper", cfg.ASR.Provider)
	}
	if cfg.ObjectStorage.Endpoint != "http://minio:9000" {
		t.Fatalf("ObjectStorage.Endpoint = %q", cfg.ObjectStorage.Endpoint)
	}
	if cfg.Tenant.DefaultID != "tenant-a" {
		t.Fatalf("Tenant.DefaultID = %q, want tenant-a", cfg.Tenant.DefaultID)
	}
}

func TestLoadAppliesEnvironmentOverrides(t *testing.T) {
	path := writeConfig(t, `
server:
  port: 9090
log:
  level: info
llm:
  api_key: yaml-key
db:
  dsn: yaml-dsn
tts:
  provider: local
asr:
  provider: local
object_storage:
  endpoint: yaml-endpoint
tenant:
  default_id: yaml-tenant
`)

	t.Setenv("SERVER_PORT", "10080")
	t.Setenv("LOG_LEVEL", "warn")
	t.Setenv("LLM_API_KEY", "env-key")
	t.Setenv("DB_DSN", "env-dsn")
	t.Setenv("TTS_PROVIDER", "elevenlabs")
	t.Setenv("ASR_PROVIDER", "deepgram")
	t.Setenv("OBJECT_STORAGE_ENDPOINT", "https://storage.example")
	t.Setenv("TENANT_DEFAULT_ID", "env-tenant")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 10080 {
		t.Fatalf("Server.Port = %d, want 10080", cfg.Server.Port)
	}
	if cfg.Log.Level != "warn" {
		t.Fatalf("Log.Level = %q, want warn", cfg.Log.Level)
	}
	if cfg.LLM.APIKey != "env-key" {
		t.Fatalf("LLM.APIKey = %q, want env-key", cfg.LLM.APIKey)
	}
	if cfg.DB.DSN != "env-dsn" {
		t.Fatalf("DB.DSN = %q, want env-dsn", cfg.DB.DSN)
	}
	if cfg.TTS.Provider != "elevenlabs" {
		t.Fatalf("TTS.Provider = %q, want elevenlabs", cfg.TTS.Provider)
	}
	if cfg.ASR.Provider != "deepgram" {
		t.Fatalf("ASR.Provider = %q, want deepgram", cfg.ASR.Provider)
	}
	if cfg.ObjectStorage.Endpoint != "https://storage.example" {
		t.Fatalf("ObjectStorage.Endpoint = %q", cfg.ObjectStorage.Endpoint)
	}
	if cfg.Tenant.DefaultID != "env-tenant" {
		t.Fatalf("Tenant.DefaultID = %q, want env-tenant", cfg.Tenant.DefaultID)
	}
}

func TestLoadAppliesPrefixedEnvironmentOverrides(t *testing.T) {
	path := writeConfig(t, `
server:
  port: 9090
tenant:
  default_id: yaml-tenant
`)

	t.Setenv("DIGITAL_TWIN_SERVER_PORT", "10081")
	t.Setenv("DIGITAL_TWIN_TENANT_DEFAULT_ID", "prefixed-tenant")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 10081 {
		t.Fatalf("Server.Port = %d, want 10081", cfg.Server.Port)
	}
	if cfg.Tenant.DefaultID != "prefixed-tenant" {
		t.Fatalf("Tenant.DefaultID = %q, want prefixed-tenant", cfg.Tenant.DefaultID)
	}
}

func TestLoadRejectsUnknownConfigKey(t *testing.T) {
	path := writeConfig(t, `
server:
  ports: 9090
`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), `unknown config key "server.ports"`) {
		t.Fatalf("Load() error = %v, want unknown config key", err)
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	path := writeConfig(t, `
server:
  port: 70000
`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "server.port must be between 1 and 65535") {
		t.Fatalf("Load() error = %v, want invalid port", err)
	}
}

func TestLoadHandlesCommentsAndQuotedValues(t *testing.T) {
	path := writeConfig(t, `
server:
  port: 8081 # trailing comment
llm:
  api_key: "key#not-comment"
tenant:
  default_id: 'tenant-a'
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 8081 {
		t.Fatalf("Server.Port = %d, want 8081", cfg.Server.Port)
	}
	if cfg.LLM.APIKey != "key#not-comment" {
		t.Fatalf("LLM.APIKey = %q, want key#not-comment", cfg.LLM.APIKey)
	}
	if cfg.Tenant.DefaultID != "tenant-a" {
		t.Fatalf("Tenant.DefaultID = %q, want tenant-a", cfg.Tenant.DefaultID)
	}
}

func TestLoadDefaultConfigFile(t *testing.T) {
	clearConfigEnv(t)

	cfg, err := Load(filepath.Join("..", "..", "configs", "app.yaml"))
	if err != nil {
		t.Fatalf("Load(default app.yaml) error = %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Fatalf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Log.Level != "info" {
		t.Fatalf("Log.Level = %q, want info", cfg.Log.Level)
	}
	if cfg.TTS.Provider != "local" || cfg.ASR.Provider != "local" {
		t.Fatalf("providers = %q/%q, want local/local", cfg.TTS.Provider, cfg.ASR.Provider)
	}
	if cfg.Tenant.DefaultID != "default" {
		t.Fatalf("Tenant.DefaultID = %q, want default", cfg.Tenant.DefaultID)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "app.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	clearConfigEnv(t)
	return path
}

func clearConfigEnv(t *testing.T) {
	t.Helper()

	names := []string{
		"DIGITAL_TWIN_SERVER_PORT", "SERVER_PORT",
		"DIGITAL_TWIN_LOG_LEVEL", "LOG_LEVEL",
		"DIGITAL_TWIN_LLM_API_KEY", "LLM_API_KEY",
		"DIGITAL_TWIN_DB_DSN", "DB_DSN",
		"DIGITAL_TWIN_TTS_PROVIDER", "TTS_PROVIDER",
		"DIGITAL_TWIN_ASR_PROVIDER", "ASR_PROVIDER",
		"DIGITAL_TWIN_OBJECT_STORAGE_ENDPOINT", "OBJECT_STORAGE_ENDPOINT",
		"DIGITAL_TWIN_TENANT_DEFAULT_ID", "TENANT_DEFAULT_ID",
	}
	for _, name := range names {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
}

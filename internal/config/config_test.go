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
  api_key: yaml-api-key
  rate_limit_requests: 12
log:
  level: debug
llm:
  provider: openai-compatible
  base_url: https://llm.example.test/v1
  model: gpt-test
  timeout_ms: 2500
  fallback_policy: fail_closed
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
  default_user_id: user-a
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Fatalf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Server.APIKey != "yaml-api-key" {
		t.Fatalf("Server.APIKey = %q, want yaml-api-key", cfg.Server.APIKey)
	}
	if cfg.Server.RateLimitRequests != 12 {
		t.Fatalf("Server.RateLimitRequests = %d, want 12", cfg.Server.RateLimitRequests)
	}
	if cfg.Log.Level != "debug" {
		t.Fatalf("Log.Level = %q, want debug", cfg.Log.Level)
	}
	if cfg.LLM.APIKey != "yaml-key" {
		t.Fatalf("LLM.APIKey = %q, want yaml-key", cfg.LLM.APIKey)
	}
	if cfg.LLM.Provider != "openai-compatible" {
		t.Fatalf("LLM.Provider = %q, want openai-compatible", cfg.LLM.Provider)
	}
	if cfg.LLM.BaseURL != "https://llm.example.test/v1" {
		t.Fatalf("LLM.BaseURL = %q, want https://llm.example.test/v1", cfg.LLM.BaseURL)
	}
	if cfg.LLM.Model != "gpt-test" {
		t.Fatalf("LLM.Model = %q, want gpt-test", cfg.LLM.Model)
	}
	if cfg.LLM.TimeoutMS != 2500 {
		t.Fatalf("LLM.TimeoutMS = %d, want 2500", cfg.LLM.TimeoutMS)
	}
	if cfg.LLM.FallbackPolicy != "fail_closed" {
		t.Fatalf("LLM.FallbackPolicy = %q, want fail_closed", cfg.LLM.FallbackPolicy)
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
	if cfg.Tenant.DefaultUserID != "user-a" {
		t.Fatalf("Tenant.DefaultUserID = %q, want user-a", cfg.Tenant.DefaultUserID)
	}
}

func TestLoadAcceptsUTF8BOM(t *testing.T) {
	path := writeConfig(t, "\ufeffserver:\n  port: 9091\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Port != 9091 {
		t.Fatalf("Server.Port = %d, want 9091", cfg.Server.Port)
	}
}

func TestLoadAppliesEnvironmentOverrides(t *testing.T) {
	path := writeConfig(t, `
server:
  port: 9090
log:
  level: info
llm:
  provider: local
  base_url: https://yaml-llm.example.test/v1
  model: yaml-model
  timeout_ms: 1500
  fallback_policy: fallback_to_local
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
  default_user_id: yaml-user
`)

	t.Setenv("SERVER_PORT", "10080")
	t.Setenv("SERVER_API_KEY", "env-api-key")
	t.Setenv("SERVER_RATE_LIMIT_REQUESTS", "7")
	t.Setenv("LOG_LEVEL", "warn")
	t.Setenv("LLM_PROVIDER", "openai-compatible")
	t.Setenv("LLM_BASE_URL", "https://env-llm.example.test/v1")
	t.Setenv("LLM_MODEL", "env-model")
	t.Setenv("LLM_TIMEOUT_MS", "3200")
	t.Setenv("LLM_FALLBACK_POLICY", "fail_closed")
	t.Setenv("LLM_API_KEY", "env-key")
	t.Setenv("DB_DSN", "env-dsn")
	t.Setenv("TTS_PROVIDER", "elevenlabs")
	t.Setenv("ASR_PROVIDER", "deepgram")
	t.Setenv("OBJECT_STORAGE_ENDPOINT", "https://storage.example")
	t.Setenv("TENANT_DEFAULT_ID", "env-tenant")
	t.Setenv("TENANT_DEFAULT_USER_ID", "env-user")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 10080 {
		t.Fatalf("Server.Port = %d, want 10080", cfg.Server.Port)
	}
	if cfg.Server.APIKey != "env-api-key" {
		t.Fatalf("Server.APIKey = %q, want env-api-key", cfg.Server.APIKey)
	}
	if cfg.Server.RateLimitRequests != 7 {
		t.Fatalf("Server.RateLimitRequests = %d, want 7", cfg.Server.RateLimitRequests)
	}
	if cfg.Log.Level != "warn" {
		t.Fatalf("Log.Level = %q, want warn", cfg.Log.Level)
	}
	if cfg.LLM.Provider != "openai-compatible" {
		t.Fatalf("LLM.Provider = %q, want openai-compatible", cfg.LLM.Provider)
	}
	if cfg.LLM.BaseURL != "https://env-llm.example.test/v1" {
		t.Fatalf("LLM.BaseURL = %q, want https://env-llm.example.test/v1", cfg.LLM.BaseURL)
	}
	if cfg.LLM.Model != "env-model" {
		t.Fatalf("LLM.Model = %q, want env-model", cfg.LLM.Model)
	}
	if cfg.LLM.TimeoutMS != 3200 {
		t.Fatalf("LLM.TimeoutMS = %d, want 3200", cfg.LLM.TimeoutMS)
	}
	if cfg.LLM.FallbackPolicy != "fail_closed" {
		t.Fatalf("LLM.FallbackPolicy = %q, want fail_closed", cfg.LLM.FallbackPolicy)
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
	if cfg.Tenant.DefaultUserID != "env-user" {
		t.Fatalf("Tenant.DefaultUserID = %q, want env-user", cfg.Tenant.DefaultUserID)
	}
}

func TestLoadAppliesPrefixedEnvironmentOverrides(t *testing.T) {
	path := writeConfig(t, `
server:
  port: 9090
tenant:
  default_id: yaml-tenant
  default_user_id: yaml-user
`)

	t.Setenv("DIGITAL_TWIN_SERVER_PORT", "10081")
	t.Setenv("DIGITAL_TWIN_TENANT_DEFAULT_ID", "prefixed-tenant")
	t.Setenv("DIGITAL_TWIN_TENANT_DEFAULT_USER_ID", "prefixed-user")

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
	if cfg.Tenant.DefaultUserID != "prefixed-user" {
		t.Fatalf("Tenant.DefaultUserID = %q, want prefixed-user", cfg.Tenant.DefaultUserID)
	}
}

func TestLoadProductionLikeProviderConfig(t *testing.T) {
	path := writeConfig(t, `
environment: production
server:
  port: 9090
llm:
  provider: openai-compatible
  base_url: https://llm.example.test/v1
  model: gpt-prod
  api_key: yaml-llm-key
tts:
  provider: http
  base_url: https://tts.example.test/v1/speech
  api_key: yaml-tts-key
asr:
  provider: local
tenant:
  default_id: tenant-prod
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Environment != "production" {
		t.Fatalf("Environment = %q, want production", cfg.Environment)
	}
	if cfg.LLM.Provider != "openai-compatible" {
		t.Fatalf("LLM.Provider = %q, want openai-compatible", cfg.LLM.Provider)
	}
	if cfg.LLM.BaseURL != "https://llm.example.test/v1" {
		t.Fatalf("LLM.BaseURL = %q", cfg.LLM.BaseURL)
	}
	if cfg.LLM.Model != "gpt-prod" {
		t.Fatalf("LLM.Model = %q, want gpt-prod", cfg.LLM.Model)
	}
	if cfg.LLM.APIKey != "yaml-llm-key" {
		t.Fatalf("LLM.APIKey = %q, want yaml-llm-key", cfg.LLM.APIKey)
	}
	if cfg.TTS.Provider != "http" {
		t.Fatalf("TTS.Provider = %q, want http", cfg.TTS.Provider)
	}
	if cfg.TTS.BaseURL != "https://tts.example.test/v1/speech" {
		t.Fatalf("TTS.BaseURL = %q", cfg.TTS.BaseURL)
	}
	if cfg.TTS.APIKey != "yaml-tts-key" {
		t.Fatalf("TTS.APIKey = %q, want yaml-tts-key", cfg.TTS.APIKey)
	}
}

func TestLoadAppliesProviderEnvironmentOverrides(t *testing.T) {
	path := writeConfig(t, `
environment: local
tts:
  provider: local
  base_url: http://yaml-tts
  api_key: yaml-tts-key
`)

	t.Setenv("DIGITAL_TWIN_ENVIRONMENT", "production")
	t.Setenv("DIGITAL_TWIN_TTS_PROVIDER", "http")
	t.Setenv("DIGITAL_TWIN_TTS_BASE_URL", "https://env-tts.example.test")
	t.Setenv("DIGITAL_TWIN_TTS_API_KEY", "env-tts-key")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Environment != "production" {
		t.Fatalf("Environment = %q, want production", cfg.Environment)
	}
	if cfg.TTS.Provider != "http" {
		t.Fatalf("TTS.Provider = %q, want http", cfg.TTS.Provider)
	}
	if cfg.TTS.BaseURL != "https://env-tts.example.test" {
		t.Fatalf("TTS.BaseURL = %q", cfg.TTS.BaseURL)
	}
	if cfg.TTS.APIKey != "env-tts-key" {
		t.Fatalf("TTS.APIKey = %q, want env-tts-key", cfg.TTS.APIKey)
	}
}

func TestLoadRejectsProductionHTTPProviderWithoutSecretLeak(t *testing.T) {
	path := writeConfig(t, `
environment: production
tts:
  provider: http
  base_url: https://tts.example.test
  api_key: super-secret-tts-key
asr:
  provider: http
  api_key: super-secret-asr-key
`)

	_, err := Load(path)
	if err == nil {
		t.Fatalf("Load() error = nil, want missing provider setting")
	}

	message := err.Error()
	if !strings.Contains(message, "asr.base_url is required") {
		t.Fatalf("Load() error = %q, want missing asr.base_url", message)
	}
	if strings.Contains(message, "super-secret-tts-key") || strings.Contains(message, "super-secret-asr-key") {
		t.Fatalf("Load() error leaked secret: %q", message)
	}
}

func TestLoadRejectsProductionLikeLLMConfigWithoutRequiredFields(t *testing.T) {
	path := writeConfig(t, `
environment: production
llm:
  provider: openai-compatible
  api_key: super-secret-llm-key
`)

	_, err := Load(path)
	if err == nil {
		t.Fatalf("Load() error = nil, want missing llm setting")
	}

	message := err.Error()
	if !strings.Contains(message, "llm.base_url is required") {
		t.Fatalf("Load() error = %q, want missing llm.base_url", message)
	}
	if strings.Contains(message, "super-secret-llm-key") {
		t.Fatalf("Load() error leaked secret: %q", message)
	}
}

func TestLoadRejectsProductionLikeLLMConfigWithoutModel(t *testing.T) {
	path := writeConfig(t, `
environment: staging
llm:
  provider: openai-compatible
  base_url: https://llm.example.test/v1
  api_key: secret
`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "llm.model is required") {
		t.Fatalf("Load() error = %v, want missing llm.model", err)
	}
}

func TestLoadRejectsProductionLikeLLMConfigWithoutAPIKey(t *testing.T) {
	path := writeConfig(t, `
environment: production
llm:
  provider: openai-compatible
  base_url: https://llm.example.test/v1
  model: gpt-prod
`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "llm.api_key is required") {
		t.Fatalf("Load() error = %v, want missing llm.api_key", err)
	}
}

func TestLoadRejectsUnsupportedLLMProvider(t *testing.T) {
	path := writeConfig(t, `
llm:
  provider: unsupported
`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "llm.provider must be one of") {
		t.Fatalf("Load() error = %v, want supported provider list", err)
	}
}

func TestLoadRejectsUnsupportedLLMFallbackPolicy(t *testing.T) {
	path := writeConfig(t, `
llm:
  fallback_policy: sometimes
`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "llm.fallback_policy must be one of") {
		t.Fatalf("Load() error = %v, want supported fallback policy list", err)
	}
}

func TestSafeSummaryRedactsSecrets(t *testing.T) {
	cfg := AppConfig{
		Environment: "production",
		Server: ServerConfig{
			Port:   8080,
			APIKey: "server-secret",
		},
		Log: LogConfig{Level: "info"},
		LLM: LLMConfig{
			Provider:       "openai-compatible",
			BaseURL:        "https://user:pass@llm.example.test/v1/chat?token=secret",
			Model:          "gpt-secret",
			TimeoutMS:      2000,
			FallbackPolicy: "fail_closed",
			APIKey:         "llm-secret",
		},
		TTS: ProviderConfig{
			Provider: "http",
			BaseURL:  "https://tts.example.test",
			APIKey:   "tts-secret",
		},
		ASR: ProviderConfig{
			Provider: "local",
			APIKey:   "asr-secret",
		},
		Tenant: TenantConfig{DefaultID: "tenant-a"},
	}

	summary := cfg.SafeSummary()

	for _, secret := range []string{"server-secret", "llm-secret", "tts-secret", "asr-secret"} {
		if strings.Contains(summary, secret) {
			t.Fatalf("SafeSummary() leaked %q in %q", secret, summary)
		}
	}
	for _, want := range []string{
		"environment=production",
		"server.port=8080",
		"llm.provider=openai-compatible",
		"llm.base_url=https://llm.example.test/v1/chat",
		"llm.model=gpt-secret",
		"llm.timeout_ms=2000",
		"llm.fallback_policy=fail_closed",
		"tts.provider=http",
		"tts.api_key=<redacted>",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("SafeSummary() = %q, want %q", summary, want)
		}
	}
}

func TestSafeSummaryRedactsProviderURLCredentials(t *testing.T) {
	cfg := AppConfig{
		TTS: ProviderConfig{
			Provider: "http",
			BaseURL:  "https://user:pass@tts.example.test/speech?api_key=url-secret&voice=pro",
		},
	}

	summary := cfg.SafeSummary()

	for _, secret := range []string{"user:pass", "url-secret", "voice=pro"} {
		if strings.Contains(summary, secret) {
			t.Fatalf("SafeSummary() leaked URL credential %q in %q", secret, summary)
		}
	}
	if !strings.Contains(summary, "tts.base_url=https://tts.example.test/speech") {
		t.Fatalf("SafeSummary() = %q, want sanitized provider URL", summary)
	}
}

func TestRedactSecretsMasksKnownSecretValues(t *testing.T) {
	cfg := AppConfig{
		Server: ServerConfig{APIKey: "server-secret"},
		LLM:    LLMConfig{APIKey: "llm-secret"},
		TTS:    ProviderConfig{APIKey: "tts-secret"},
		ASR:    ProviderConfig{APIKey: "asr-secret"},
	}

	text := "Authorization: Bearer server-secret llm=llm-secret tts=tts-secret asr=asr-secret"
	redacted := cfg.RedactSecrets(text)

	for _, secret := range []string{"server-secret", "llm-secret", "tts-secret", "asr-secret"} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("RedactSecrets() leaked %q in %q", secret, redacted)
		}
	}
	if strings.Count(redacted, "<redacted>") != 4 {
		t.Fatalf("RedactSecrets() = %q, want four redactions", redacted)
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

func TestLoadDefaultsServerHostToLoopback(t *testing.T) {
	path := writeConfig(t, `
environment: local
server:
  port: 8080
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("Server.Host = %q, want 127.0.0.1", cfg.Server.Host)
	}
}

func TestLoadReadsServerHostFromEnv(t *testing.T) {
	path := writeConfig(t, `server:
  port: 8080
`)
	t.Setenv("DIGITAL_TWIN_SERVER_HOST", "0.0.0.0")
	t.Setenv("DIGITAL_TWIN_SERVER_API_KEY", "env-api-key")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Fatalf("Server.Host = %q, want 0.0.0.0", cfg.Server.Host)
	}
}

func TestLoadRejectsNonLocalServerHostWithoutAPIKey(t *testing.T) {
	path := writeConfig(t, `
environment: local
server:
  host: 0.0.0.0
  port: 8080
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected non-local host without api key to be rejected")
	}
	if !strings.Contains(err.Error(), "server.api_key is required when server.host is non-local") {
		t.Fatalf("error = %v", err)
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
  default_user_id: 'user-a'
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
	if cfg.Tenant.DefaultUserID != "user-a" {
		t.Fatalf("Tenant.DefaultUserID = %q, want user-a", cfg.Tenant.DefaultUserID)
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
	if cfg.Tenant.DefaultUserID != "default-user" {
		t.Fatalf("Tenant.DefaultUserID = %q, want default-user", cfg.Tenant.DefaultUserID)
	}
}

func TestLoadMinimalConfigDefaultsLLMToLocal(t *testing.T) {
	path := writeConfig(t, `
server:
  port: 8080
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.LLM.Provider != "local" {
		t.Fatalf("LLM.Provider = %q, want local", cfg.LLM.Provider)
	}
	if cfg.LLM.FallbackPolicy != "fallback_to_local" {
		t.Fatalf("LLM.FallbackPolicy = %q, want fallback_to_local", cfg.LLM.FallbackPolicy)
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
		"DIGITAL_TWIN_SERVER_HOST", "SERVER_HOST",
		"DIGITAL_TWIN_SERVER_API_KEY", "SERVER_API_KEY",
		"DIGITAL_TWIN_SERVER_RATE_LIMIT_REQUESTS", "SERVER_RATE_LIMIT_REQUESTS",
		"DIGITAL_TWIN_LOG_LEVEL", "LOG_LEVEL",
		"DIGITAL_TWIN_ENVIRONMENT", "DIGITAL_TWIN_ENV",
		"DIGITAL_TWIN_LLM_PROVIDER", "LLM_PROVIDER",
		"DIGITAL_TWIN_LLM_BASE_URL", "LLM_BASE_URL",
		"DIGITAL_TWIN_LLM_MODEL", "LLM_MODEL",
		"DIGITAL_TWIN_LLM_TIMEOUT_MS", "LLM_TIMEOUT_MS",
		"DIGITAL_TWIN_LLM_FALLBACK_POLICY", "LLM_FALLBACK_POLICY",
		"DIGITAL_TWIN_LLM_API_KEY", "LLM_API_KEY",
		"DIGITAL_TWIN_DB_DSN", "DB_DSN",
		"DIGITAL_TWIN_TTS_PROVIDER", "TTS_PROVIDER",
		"DIGITAL_TWIN_TTS_BASE_URL", "TTS_BASE_URL",
		"DIGITAL_TWIN_TTS_API_KEY", "TTS_API_KEY",
		"DIGITAL_TWIN_ASR_PROVIDER", "ASR_PROVIDER",
		"DIGITAL_TWIN_ASR_BASE_URL", "ASR_BASE_URL",
		"DIGITAL_TWIN_ASR_API_KEY", "ASR_API_KEY",
		"DIGITAL_TWIN_OBJECT_STORAGE_ENDPOINT", "OBJECT_STORAGE_ENDPOINT",
		"DIGITAL_TWIN_TENANT_DEFAULT_ID", "TENANT_DEFAULT_ID",
	}
	for _, name := range names {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("unset %s: %v", name, err)
		}
	}
}

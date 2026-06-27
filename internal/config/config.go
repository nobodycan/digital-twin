package config

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type AppConfig struct {
	Environment   string
	Server        ServerConfig
	Log           LogConfig
	LLM           LLMConfig
	DB            DBConfig
	TTS           ProviderConfig
	ASR           ProviderConfig
	ObjectStorage ObjectStorageConfig
	Tenant        TenantConfig
}

type ServerConfig struct {
	Port              int
	APIKey            string
	RateLimitRequests int
}

type LogConfig struct {
	Level string
}

type LLMConfig struct {
	Provider       string
	BaseURL        string
	Model          string
	TimeoutMS      int
	FallbackPolicy string
	APIKey         string
}

type DBConfig struct {
	DSN string
}

type ProviderConfig struct {
	Provider string
	BaseURL  string
	APIKey   string
}

type ObjectStorageConfig struct {
	Endpoint string
}

type TenantConfig struct {
	DefaultID     string
	DefaultUserID string
}

func Load(path string) (AppConfig, error) {
	values, err := parseYAMLFile(path)
	if err != nil {
		return AppConfig{}, err
	}

	cfg := AppConfig{
		Environment:   "local",
		Server:        ServerConfig{Port: 8080},
		Log:           LogConfig{Level: "info"},
		LLM:           LLMConfig{Provider: "local", FallbackPolicy: "fallback_to_local"},
		TTS:           ProviderConfig{Provider: "local"},
		ASR:           ProviderConfig{Provider: "local"},
		Tenant:        TenantConfig{DefaultID: "default", DefaultUserID: "default-user"},
		ObjectStorage: ObjectStorageConfig{},
	}

	if err := applyValues(&cfg, values); err != nil {
		return AppConfig{}, err
	}
	if err := applyEnv(&cfg); err != nil {
		return AppConfig{}, err
	}
	if err := validate(cfg); err != nil {
		return AppConfig{}, err
	}

	return cfg, nil
}

func parseYAMLFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	values := make(map[string]string)
	var sections []string
	scanner := bufio.NewScanner(file)
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		if lineNo == 1 {
			raw = strings.TrimPrefix(raw, "\ufeff")
		}
		line := stripComment(raw)
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.Contains(line, "\t") {
			return nil, fmt.Errorf("parse config line %d: tabs are not supported", lineNo)
		}

		indent := countLeadingSpaces(line)
		if indent%2 != 0 {
			return nil, fmt.Errorf("parse config line %d: indentation must use two spaces", lineNo)
		}

		trimmed := strings.TrimSpace(line)
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("parse config line %d: expected key: value", lineNo)
		}

		level := indent / 2
		if level > len(sections) {
			return nil, fmt.Errorf("parse config line %d: unexpected indentation", lineNo)
		}
		sections = sections[:level]

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if value == "" {
			sections = append(sections, key)
			continue
		}

		pathParts := append(append([]string{}, sections...), key)
		values[strings.Join(pathParts, ".")] = unquote(value)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	return values, nil
}

func applyValues(cfg *AppConfig, values map[string]string) error {
	for key, value := range values {
		if err := setValue(cfg, key, value); err != nil {
			return err
		}
	}
	return nil
}

func applyEnv(cfg *AppConfig) error {
	env := map[string]string{
		"environment":                firstEnv("DIGITAL_TWIN_ENVIRONMENT", "DIGITAL_TWIN_ENV"),
		"server.port":                firstEnv("DIGITAL_TWIN_SERVER_PORT", "SERVER_PORT"),
		"server.api_key":             firstEnv("DIGITAL_TWIN_SERVER_API_KEY", "SERVER_API_KEY"),
		"server.rate_limit_requests": firstEnv("DIGITAL_TWIN_SERVER_RATE_LIMIT_REQUESTS", "SERVER_RATE_LIMIT_REQUESTS"),
		"log.level":                  firstEnv("DIGITAL_TWIN_LOG_LEVEL", "LOG_LEVEL"),
		"llm.provider":               firstEnv("DIGITAL_TWIN_LLM_PROVIDER", "LLM_PROVIDER"),
		"llm.base_url":               firstEnv("DIGITAL_TWIN_LLM_BASE_URL", "LLM_BASE_URL"),
		"llm.model":                  firstEnv("DIGITAL_TWIN_LLM_MODEL", "LLM_MODEL"),
		"llm.timeout_ms":             firstEnv("DIGITAL_TWIN_LLM_TIMEOUT_MS", "LLM_TIMEOUT_MS"),
		"llm.fallback_policy":        firstEnv("DIGITAL_TWIN_LLM_FALLBACK_POLICY", "LLM_FALLBACK_POLICY"),
		"llm.api_key":                firstEnv("DIGITAL_TWIN_LLM_API_KEY", "LLM_API_KEY"),
		"db.dsn":                     firstEnv("DIGITAL_TWIN_DB_DSN", "DB_DSN"),
		"tts.provider":               firstEnv("DIGITAL_TWIN_TTS_PROVIDER", "TTS_PROVIDER"),
		"tts.base_url":               firstEnv("DIGITAL_TWIN_TTS_BASE_URL", "TTS_BASE_URL"),
		"tts.api_key":                firstEnv("DIGITAL_TWIN_TTS_API_KEY", "TTS_API_KEY"),
		"asr.provider":               firstEnv("DIGITAL_TWIN_ASR_PROVIDER", "ASR_PROVIDER"),
		"asr.base_url":               firstEnv("DIGITAL_TWIN_ASR_BASE_URL", "ASR_BASE_URL"),
		"asr.api_key":                firstEnv("DIGITAL_TWIN_ASR_API_KEY", "ASR_API_KEY"),
		"object_storage.endpoint":    firstEnv("DIGITAL_TWIN_OBJECT_STORAGE_ENDPOINT", "OBJECT_STORAGE_ENDPOINT"),
		"tenant.default_id":          firstEnv("DIGITAL_TWIN_TENANT_DEFAULT_ID", "TENANT_DEFAULT_ID"),
		"tenant.default_user_id":     firstEnv("DIGITAL_TWIN_TENANT_DEFAULT_USER_ID", "TENANT_DEFAULT_USER_ID"),
	}

	for key, value := range env {
		if value == "" {
			continue
		}
		if err := setValue(cfg, key, value); err != nil {
			return err
		}
	}
	return nil
}

func setValue(cfg *AppConfig, key, value string) error {
	switch key {
	case "environment":
		cfg.Environment = value
	case "server.port":
		port, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse server.port: %w", err)
		}
		cfg.Server.Port = port
	case "server.api_key":
		cfg.Server.APIKey = value
	case "server.rate_limit_requests":
		limit, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse server.rate_limit_requests: %w", err)
		}
		cfg.Server.RateLimitRequests = limit
	case "log.level":
		cfg.Log.Level = value
	case "llm.provider":
		cfg.LLM.Provider = value
	case "llm.base_url":
		cfg.LLM.BaseURL = value
	case "llm.model":
		cfg.LLM.Model = value
	case "llm.timeout_ms":
		timeoutMS, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse llm.timeout_ms: %w", err)
		}
		cfg.LLM.TimeoutMS = timeoutMS
	case "llm.fallback_policy":
		cfg.LLM.FallbackPolicy = value
	case "llm.api_key":
		cfg.LLM.APIKey = value
	case "db.dsn":
		cfg.DB.DSN = value
	case "tts.provider":
		cfg.TTS.Provider = value
	case "tts.base_url":
		cfg.TTS.BaseURL = value
	case "tts.api_key":
		cfg.TTS.APIKey = value
	case "asr.provider":
		cfg.ASR.Provider = value
	case "asr.base_url":
		cfg.ASR.BaseURL = value
	case "asr.api_key":
		cfg.ASR.APIKey = value
	case "object_storage.endpoint":
		cfg.ObjectStorage.Endpoint = value
	case "tenant.default_id":
		cfg.Tenant.DefaultID = value
	case "tenant.default_user_id":
		cfg.Tenant.DefaultUserID = value
	default:
		return fmt.Errorf("unknown config key %q", key)
	}
	return nil
}

func validate(cfg AppConfig) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}
	if err := validateLLM(cfg.Environment, cfg.LLM); err != nil {
		return err
	}
	if err := validateProvider("tts", cfg.Environment, cfg.TTS); err != nil {
		return err
	}
	if err := validateProvider("asr", cfg.Environment, cfg.ASR); err != nil {
		return err
	}
	return nil
}

func validateProvider(name, environment string, provider ProviderConfig) error {
	if strings.TrimSpace(provider.Provider) == "" {
		return fmt.Errorf("%s.provider is required", name)
	}
	if !isProductionLike(environment) || provider.Provider != "http" {
		return nil
	}
	if strings.TrimSpace(provider.BaseURL) == "" {
		return fmt.Errorf("%s.base_url is required for http provider in %s", name, environment)
	}
	if strings.TrimSpace(provider.APIKey) == "" {
		return fmt.Errorf("%s.api_key is required for http provider in %s", name, environment)
	}
	return nil
}

func validateLLM(environment string, llm LLMConfig) error {
	if strings.TrimSpace(llm.Provider) == "" {
		return fmt.Errorf("llm.provider is required")
	}
	switch llm.Provider {
	case "local", "mock", "openai-compatible":
	default:
		return fmt.Errorf("llm.provider must be one of local, mock, openai-compatible")
	}
	if llm.TimeoutMS < 0 {
		return fmt.Errorf("llm.timeout_ms must be greater than or equal to 0")
	}
	if strings.TrimSpace(llm.FallbackPolicy) == "" {
		return fmt.Errorf("llm.fallback_policy is required")
	}
	switch llm.FallbackPolicy {
	case "fallback_to_local", "fail_closed":
	default:
		return fmt.Errorf("llm.fallback_policy must be one of fallback_to_local, fail_closed")
	}
	if !isProductionLike(environment) || llm.Provider != "openai-compatible" {
		return nil
	}
	if strings.TrimSpace(llm.BaseURL) == "" {
		return fmt.Errorf("llm.base_url is required for openai-compatible provider in %s", environment)
	}
	if strings.TrimSpace(llm.Model) == "" {
		return fmt.Errorf("llm.model is required for openai-compatible provider in %s", environment)
	}
	if strings.TrimSpace(llm.APIKey) == "" {
		return fmt.Errorf("llm.api_key is required for openai-compatible provider in %s", environment)
	}
	return nil
}

func (cfg AppConfig) SafeSummary() string {
	fields := []string{
		"environment=" + cfg.Environment,
		"server.port=" + strconv.Itoa(cfg.Server.Port),
		"log.level=" + cfg.Log.Level,
		"tenant.default_id=" + cfg.Tenant.DefaultID,
		"tenant.default_user_id=" + cfg.Tenant.DefaultUserID,
		"llm.provider=" + cfg.LLM.Provider,
		"llm.base_url=" + safeURLSummary(cfg.LLM.BaseURL),
		"llm.model=" + cfg.LLM.Model,
		"llm.timeout_ms=" + strconv.Itoa(cfg.LLM.TimeoutMS),
		"llm.fallback_policy=" + cfg.LLM.FallbackPolicy,
		"llm.api_key=" + redactedMarker(cfg.LLM.APIKey),
		"tts.provider=" + cfg.TTS.Provider,
		"tts.base_url=" + safeURLSummary(cfg.TTS.BaseURL),
		"tts.api_key=" + redactedMarker(cfg.TTS.APIKey),
		"asr.provider=" + cfg.ASR.Provider,
		"asr.base_url=" + safeURLSummary(cfg.ASR.BaseURL),
		"asr.api_key=" + redactedMarker(cfg.ASR.APIKey),
		"server.api_key=" + redactedMarker(cfg.Server.APIKey),
	}
	return strings.Join(fields, " ")
}

func (cfg AppConfig) RedactSecrets(text string) string {
	redacted := text
	for _, secret := range cfg.secretValues() {
		redacted = strings.ReplaceAll(redacted, secret, "<redacted>")
	}
	return redacted
}

func (cfg AppConfig) secretValues() []string {
	candidates := []string{
		cfg.Server.APIKey,
		cfg.LLM.APIKey,
		cfg.TTS.APIKey,
		cfg.ASR.APIKey,
	}
	secrets := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) != "" {
			secrets = append(secrets, candidate)
		}
	}
	return secrets
}

func redactedMarker(secret string) string {
	if strings.TrimSpace(secret) == "" {
		return ""
	}
	return "<redacted>"
}

func safeURLSummary(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return "<redacted-url>"
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func isProductionLike(environment string) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "production", "prod", "staging", "stage":
		return true
	default:
		return false
	}
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value, ok := os.LookupEnv(name); ok {
			return value
		}
	}
	return ""
}

func stripComment(line string) string {
	inSingle := false
	inDouble := false
	for i, r := range line {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return strings.TrimRight(line[:i], " ")
			}
		}
	}
	return line
}

func countLeadingSpaces(line string) int {
	count := 0
	for _, r := range line {
		if r != ' ' {
			break
		}
		count++
	}
	return count
}

func unquote(value string) string {
	if len(value) < 2 {
		return value
	}
	if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
		return value[1 : len(value)-1]
	}
	return value
}

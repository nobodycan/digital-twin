package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type AppConfig struct {
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
	Port int
}

type LogConfig struct {
	Level string
}

type LLMConfig struct {
	APIKey string
}

type DBConfig struct {
	DSN string
}

type ProviderConfig struct {
	Provider string
}

type ObjectStorageConfig struct {
	Endpoint string
}

type TenantConfig struct {
	DefaultID string
}

func Load(path string) (AppConfig, error) {
	values, err := parseYAMLFile(path)
	if err != nil {
		return AppConfig{}, err
	}

	cfg := AppConfig{
		Server:        ServerConfig{Port: 8080},
		Log:           LogConfig{Level: "info"},
		TTS:           ProviderConfig{Provider: "local"},
		ASR:           ProviderConfig{Provider: "local"},
		Tenant:        TenantConfig{DefaultID: "default"},
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
		"server.port":             firstEnv("DIGITAL_TWIN_SERVER_PORT", "SERVER_PORT"),
		"log.level":               firstEnv("DIGITAL_TWIN_LOG_LEVEL", "LOG_LEVEL"),
		"llm.api_key":             firstEnv("DIGITAL_TWIN_LLM_API_KEY", "LLM_API_KEY"),
		"db.dsn":                  firstEnv("DIGITAL_TWIN_DB_DSN", "DB_DSN"),
		"tts.provider":            firstEnv("DIGITAL_TWIN_TTS_PROVIDER", "TTS_PROVIDER"),
		"asr.provider":            firstEnv("DIGITAL_TWIN_ASR_PROVIDER", "ASR_PROVIDER"),
		"object_storage.endpoint": firstEnv("DIGITAL_TWIN_OBJECT_STORAGE_ENDPOINT", "OBJECT_STORAGE_ENDPOINT"),
		"tenant.default_id":       firstEnv("DIGITAL_TWIN_TENANT_DEFAULT_ID", "TENANT_DEFAULT_ID"),
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
	case "server.port":
		port, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse server.port: %w", err)
		}
		cfg.Server.Port = port
	case "log.level":
		cfg.Log.Level = value
	case "llm.api_key":
		cfg.LLM.APIKey = value
	case "db.dsn":
		cfg.DB.DSN = value
	case "tts.provider":
		cfg.TTS.Provider = value
	case "asr.provider":
		cfg.ASR.Provider = value
	case "object_storage.endpoint":
		cfg.ObjectStorage.Endpoint = value
	case "tenant.default_id":
		cfg.Tenant.DefaultID = value
	default:
		return fmt.Errorf("unknown config key %q", key)
	}
	return nil
}

func validate(cfg AppConfig) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}
	return nil
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

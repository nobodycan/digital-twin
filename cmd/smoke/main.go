package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type smokeConfig struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

func main() {
	baseURL := flag.String("base-url", "", "base URL for the digital-twin runtime")
	apiKey := flag.String("api-key", os.Getenv("DIGITAL_TWIN_SERVER_API_KEY"), "API key for protected runtime checks")
	flag.Parse()

	if err := run(smokeConfig{BaseURL: *baseURL, APIKey: *apiKey}); err != nil {
		fmt.Fprintf(os.Stderr, "smoke failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("smoke passed")
}

func run(config smokeConfig) error {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		return errors.New("base URL is required")
	}
	client := config.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	checks := []struct {
		method string
		path   string
		body   string
		auth   bool
	}{
		{method: http.MethodGet, path: "/health"},
		{method: http.MethodGet, path: "/ready"},
		{method: http.MethodPost, path: "/chat", body: smokeConversationJSON(), auth: true},
		{method: http.MethodPost, path: "/chat/stream", body: smokeTurnRequestJSON(), auth: true},
		{method: http.MethodGet, path: "/app"},
		{method: http.MethodGet, path: "/admin"},
		{method: http.MethodGet, path: "/metrics"},
	}

	for _, check := range checks {
		if err := runCheck(ctx, client, baseURL, config.APIKey, check.method, check.path, check.body, check.auth); err != nil {
			return err
		}
	}
	return nil
}

func runCheck(ctx context.Context, client *http.Client, baseURL, apiKey, method, path, body string, auth bool) error {
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("%s %s request: %w", method, path, err)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if auth && strings.TrimSpace(apiKey) != "" {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("%s %s failed: %w", method, path, err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		sample, _ := io.ReadAll(io.LimitReader(response.Body, 256))
		return fmt.Errorf("%s %s returned %d: %s", method, path, response.StatusCode, strings.TrimSpace(string(sample)))
	}
	return nil
}

func smokeConversationJSON() string {
	now := "2026-06-22T00:00:00Z"
	return `{"id":"smoke-conv","tenant_id":"tenant-1","user_id":"smoke-user","messages":[{"id":"smoke-msg","role":"user","content":"smoke check","created_at":"` + now + `"}],"created_at":"` + now + `","updated_at":"` + now + `"}`
}

func smokeTurnRequestJSON() string {
	now := "2026-06-22T00:00:00Z"
	return `{"conversation_id":"smoke-conv","tenant_id":"tenant-1","user_id":"smoke-user","turn_id":"smoke-turn-1","attempt_id":"smoke-attempt-1","message":{"id":"smoke-msg","role":"user","content":"smoke check","created_at":"` + now + `"}}`
}

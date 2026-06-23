package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nobodycan/digital-twin/internal/config"
	"github.com/nobodycan/digital-twin/internal/observability"
)

var ErrProviderFailure = errors.New("voice provider failure")

func NewTTSClient(provider config.ProviderConfig) (TTSClient, error) {
	return NewTTSClientWithMetrics(provider, nil)
}

func NewTTSClientWithMetrics(provider config.ProviderConfig, metrics observability.Metrics) (TTSClient, error) {
	switch strings.ToLower(strings.TrimSpace(provider.Provider)) {
	case "", "local", "mock":
		return MockTTSClient{}, nil
	case "http":
		if strings.TrimSpace(provider.BaseURL) == "" {
			return nil, fmt.Errorf("tts.base_url is required for http provider")
		}
		return HTTPTTSClient{
			baseURL: strings.TrimRight(provider.BaseURL, "/"),
			apiKey:  provider.APIKey,
			client:  &http.Client{Timeout: 10 * time.Second},
			metrics: metrics,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported tts provider %q", provider.Provider)
	}
}

type HTTPTTSClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
	metrics observability.Metrics
}

func (client HTTPTTSClient) Synthesize(ctx context.Context, request TTSRequest) (TTSResult, error) {
	if strings.TrimSpace(request.Text) == "" {
		return TTSResult{}, ErrEmptyInput
	}
	start := time.Now()
	labels := map[string]string{"provider": "http", "operation": "tts"}
	defer func() {
		if client.metrics != nil {
			client.metrics.Observe("voice_provider_latency_ms", float64(time.Since(start).Milliseconds()), labels)
		}
	}()

	body, err := json.Marshal(request)
	if err != nil {
		return TTSResult{}, fmt.Errorf("encode tts request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL, bytes.NewReader(body))
	if err != nil {
		return TTSResult{}, fmt.Errorf("create tts request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(client.apiKey) != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+client.apiKey)
	}

	httpClient := client.client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	response, err := httpClient.Do(httpRequest)
	if err != nil {
		client.recordProviderError(labels)
		return TTSResult{}, fmt.Errorf("%w: request failed", ErrProviderFailure)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		client.recordProviderError(labels)
		return TTSResult{}, fmt.Errorf("%w: provider returned status %d", ErrProviderFailure, response.StatusCode)
	}

	var result TTSResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		client.recordProviderError(labels)
		return TTSResult{}, fmt.Errorf("%w: decode response", ErrProviderFailure)
	}
	if strings.TrimSpace(result.Provider) == "" {
		result.Provider = "http"
	}
	if len(result.Chunks) == 0 {
		client.recordProviderError(labels)
		return TTSResult{}, fmt.Errorf("%w: empty audio response", ErrProviderFailure)
	}
	return result, nil
}

func (client HTTPTTSClient) recordProviderError(labels map[string]string) {
	if client.metrics != nil {
		client.metrics.IncCounter("voice_provider_errors_total", labels)
	}
}

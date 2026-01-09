/*
Package provider implements LLM provider integrations.

All providers support streaming chat completions via the Query method,
which calls onChunk for each content delta.
*/
package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"

	"llm/internal/session"
)

const (
	// StreamingRequestTimeout is the timeout for streaming LLM requests
	StreamingRequestTimeout = 120 * time.Second
	// ModelDiscoveryTimeout is the timeout for OpenRouter model discovery
	ModelDiscoveryTimeout = 30 * time.Second
)

// Provider is the interface for LLM providers.
type Provider interface {
	Query(model string, messages []session.Message, onChunk func(string)) (string, error)
}

// wrapTimeoutError checks if an error is a context timeout and returns a user-friendly message
func wrapTimeoutError(err error, operation string) error {
	if err == context.DeadlineExceeded {
		return fmt.Errorf("request timed out after %v while %s - the API may be slow or unresponsive", StreamingRequestTimeout, operation)
	}
	return err
}

// handleHTTPError returns a user-friendly error message based on HTTP status code
func handleHTTPError(resp *http.Response, providerName string) error {
	body, _ := io.ReadAll(resp.Body)
	bodyStr := strings.TrimSpace(string(body))

	switch resp.StatusCode {
	case 401, 403:
		return fmt.Errorf("authentication failed: check your %s API key in ~/.llmrc or environment variables", providerName)
	case 429:
		return fmt.Errorf("rate limit exceeded: you've hit the API rate limit, please wait and try again")
	case 400:
		// Try to extract error message from response body
		var errorData map[string]interface{}
		if err := json.Unmarshal(body, &errorData); err == nil {
			if errorMsg, ok := errorData["error"].(map[string]interface{}); ok {
				if message, ok := errorMsg["message"].(string); ok {
					return fmt.Errorf("bad request: %s", message)
				}
			} else if errorMsg, ok := errorData["error"].(string); ok {
				return fmt.Errorf("bad request: %s", errorMsg)
			}
		}
		if bodyStr != "" {
			return fmt.Errorf("bad request: %s", bodyStr)
		}
		return fmt.Errorf("bad request: invalid parameters or malformed request")
	case 500, 502, 503, 504:
		return fmt.Errorf("provider server error (%d): %s service may be temporarily unavailable, try again later", resp.StatusCode, providerName)
	default:
		if bodyStr != "" {
			return fmt.Errorf("unexpected HTTP %d: %s", resp.StatusCode, bodyStr)
		}
		return fmt.Errorf("unexpected HTTP %d: %s", resp.StatusCode, resp.Status)
	}
}

// handleOpenAIError wraps OpenAI library errors with user-friendly messages
func handleOpenAIError(err error) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Check for common error patterns in the go-openai library
	if strings.Contains(errStr, "401") || strings.Contains(errStr, "Incorrect API key") || strings.Contains(errStr, "invalid_api_key") {
		return fmt.Errorf("authentication failed: check your OpenAI API key in OPENAI_API_KEY environment variable or ~/.llmrc")
	}
	if strings.Contains(errStr, "429") || strings.Contains(errStr, "rate_limit") {
		return fmt.Errorf("rate limit exceeded: you've hit the OpenAI rate limit, please wait and try again or upgrade your plan")
	}
	if strings.Contains(errStr, "400") || strings.Contains(errStr, "invalid_request") {
		return fmt.Errorf("bad request: %v", err)
	}
	if strings.Contains(errStr, "500") || strings.Contains(errStr, "502") || strings.Contains(errStr, "503") || strings.Contains(errStr, "504") {
		return fmt.Errorf("OpenAI server error: service may be temporarily unavailable, try again later")
	}

	return err
}

// OpenAIProvider is the provider for OpenAI API using go-openai library.
type OpenAIProvider struct {
	client *openai.Client
}

// NewOpenAIProvider creates a new OpenAIProvider with the given API key.
func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	client := openai.NewClient(apiKey)
	return &OpenAIProvider{client: client}
}

// Query sends the messages to OpenAI chat completions endpoint as stream.
// Converts Message to openai.ChatCompletionMessage, streams deltas via onChunk.
func (p *OpenAIProvider) Query(model string, messages []session.Message, onChunk func(string)) (string, error) {
	if model == "" {
		model = openai.GPT3Dot5Turbo
	}
	var openaiMessages []openai.ChatCompletionMessage
	for _, msg := range messages {
		var role string
		switch msg.Role {
		case "user":
			role = openai.ChatMessageRoleUser
		case "assistant":
			role = openai.ChatMessageRoleAssistant
		case "system":
			role = openai.ChatMessageRoleSystem
		}
		openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
			Role:    role,
			Content: msg.Content,
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), StreamingRequestTimeout)
	defer cancel()

	stream, err := p.client.CreateChatCompletionStream(
		ctx,
		openai.ChatCompletionRequest{
			Model:    model,
			Messages: openaiMessages,
			Stream:   true,
		},
	)
	if err != nil {
		err = wrapTimeoutError(err, "streaming from OpenAI")
		return "", handleOpenAIError(err)
	}
	defer stream.Close()

	var fullResponse string
	for {
		response, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			err = wrapTimeoutError(err, "receiving OpenAI stream")
			return "", handleOpenAIError(err)
		}
		if len(response.Choices) > 0 {
			chunk := response.Choices[0].Delta.Content
			if chunk != "" {
				onChunk(chunk)
				fullResponse += chunk
			}
		}
	}
	return fullResponse, nil
}

// GrokProvider is the provider for xAI Grok API.
type GrokProvider struct {
	apiKey string
}

// NewGrokProvider creates a new GrokProvider with the given API key.
func NewGrokProvider(apiKey string) *GrokProvider {
	return &GrokProvider{apiKey: apiKey}
}

// Query sends messages to Grok chat completions endpoint as SSE stream.
// Parses SSE events for content deltas.
func (p *GrokProvider) Query(model string, messages []session.Message, onChunk func(string)) (string, error) {
	if model == "" {
		model = "grok-3"
	}
	url := "https://api.x.ai/v1/chat/completions"
	var grokMessages []map[string]string
	for _, msg := range messages {
		grokMessages = append(grokMessages, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}
	payload := map[string]interface{}{
		"messages": grokMessages,
		"model":    model,
		"stream":   true,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), StreamingRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", wrapTimeoutError(err, "connecting to Grok API")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", handleHTTPError(resp, "Grok")
	}

	var fullResponse string
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		line = strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}
			if choices, ok := event["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if delta, ok := choice["delta"].(map[string]interface{}); ok {
						if content, ok := delta["content"].(string); ok && content != "" {
							onChunk(content)
							fullResponse += content
						}
					}
				}
			}
		}
	}
	return fullResponse, nil
}

// LMProxyProvider is the provider for lm-proxy (OpenAI-compatible proxy server).
type LMProxyProvider struct {
	apiKey  string
	baseURL string
}

// NewLMProxyProvider creates a new LMProxyProvider with the given API key and base URL.
func NewLMProxyProvider(apiKey, baseURL string) *LMProxyProvider {
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}
	return &LMProxyProvider{apiKey: apiKey, baseURL: baseURL}
}

// Query sends messages to lm-proxy chat completions endpoint as SSE stream.
func (p *LMProxyProvider) Query(model string, messages []session.Message, onChunk func(string)) (string, error) {
	if model == "" {
		model = "gpt-3.5-turbo"
	}
	url := strings.TrimSuffix(p.baseURL, "/") + "/v1/chat/completions"
	var proxyMessages []map[string]string
	for _, msg := range messages {
		proxyMessages = append(proxyMessages, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}
	payload := map[string]interface{}{
		"messages": proxyMessages,
		"model":    model,
		"stream":   true,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), StreamingRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", wrapTimeoutError(err, "connecting to lm-proxy")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", handleHTTPError(resp, "lm-proxy")
	}

	var fullResponse string
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		line = strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}
			if choices, ok := event["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if delta, ok := choice["delta"].(map[string]interface{}); ok {
						if content, ok := delta["content"].(string); ok && content != "" {
							onChunk(content)
							fullResponse += content
						}
					}
				}
			}
		}
	}
	return fullResponse, nil
}

// OpenRouterProvider is the provider for OpenRouter API.
type OpenRouterProvider struct {
	apiKey   string
	FreeMode bool
}

// NewOpenRouterProvider creates a new OpenRouterProvider with the given API key.
func NewOpenRouterProvider(apiKey string) *OpenRouterProvider {
	return &OpenRouterProvider{apiKey: apiKey}
}

// Query sends messages to OpenRouter chat completions endpoint as SSE stream.
// Supports free mode with max_price=0 for input/output.
func (p *OpenRouterProvider) Query(model string, messages []session.Message, onChunk func(string)) (string, error) {
	if model == "" {
		model = "openai/gpt-3.5-turbo"
	}
	url := "https://openrouter.ai/api/v1/chat/completions"
	var routerMessages []map[string]string
	for _, msg := range messages {
		routerMessages = append(routerMessages, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}
	payload := map[string]interface{}{
		"messages": routerMessages,
		"model":    model,
		"stream":   true,
	}
	if p.FreeMode {
		payload["max_price"] = map[string]float64{
			"input":  0.0,
			"output": 0.0,
		}
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), StreamingRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", wrapTimeoutError(err, "connecting to OpenRouter API")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", handleHTTPError(resp, "OpenRouter")
	}

	var fullResponse string
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		line = strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}
			if choices, ok := event["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if delta, ok := choice["delta"].(map[string]interface{}); ok {
						if content, ok := delta["content"].(string); ok && content != "" {
							onChunk(content)
							fullResponse += content
						}
					}
				}
			}
		}
	}
	return fullResponse, nil
}

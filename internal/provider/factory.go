package provider

import (
	"fmt"

	"llm/internal/config"
)

// Factory creates Provider implementations based on provider string.
type Factory struct{}

// NewFactory returns a new Factory.
func NewFactory() *Factory {
	return &Factory{}
}

// Create instantiates a Provider for the given model string (provider name)
// and returns the provider along with the query model (with defaults).
func (f *Factory) Create(model string, cfg *config.Config) (Provider, string, error) {
	var prov Provider
	var queryModel string
	switch model {
	case "openrouter":
		prov = NewOpenRouterProvider(cfg.OpenRouterAPIKey)
		queryModel = cfg.OpenRouterModel
		if queryModel == "" {
			queryModel = "openai/gpt-3.5-turbo"
		}
	case "grok":
		prov = NewGrokProvider(cfg.GrokAPIKey)
		queryModel = cfg.GrokModel
		if queryModel == "" {
			queryModel = "grok-3"
		}
	case "openai":
		prov = NewOpenAIProvider(cfg.OpenAIAPIKey)
		queryModel = cfg.OpenAIModel
		if queryModel == "" {
			queryModel = "gpt-3.5-turbo"
		}
	case "lm-proxy":
		prov = NewLMProxyProvider(cfg.LMProxyAPIKey, cfg.LMProxyURL)
		queryModel = cfg.LMProxyModel
		if queryModel == "" {
			queryModel = "gpt-3.5-turbo"
		}
	default:
		return nil, "", fmt.Errorf("unknown model: %s", model)
	}
	return prov, queryModel, nil
}

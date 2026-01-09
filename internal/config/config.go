/*
Package config provides configuration management for the LLM CLI tool.

Config holds API keys, model names, default provider, and system prompt from ~/.llmrc.
*/
package config

import (
	"bufio"
	"os"
	"strings"
)

// Config holds the configuration for LLM providers.
// Fields are populated from key=value pairs in the config file.
type Config struct {
	OpenRouterAPIKey string
	GrokAPIKey       string
	OpenAIAPIKey     string
	LMProxyAPIKey    string
	LMProxyURL       string
	OpenRouterModel  string
	GrokModel        string
	OpenAIModel      string
	LMProxyModel     string
	DefaultProvider  string
	SystemPrompt     string
}

// Loader is the loader for configuration files.
type Loader struct{}

// NewLoader creates a new Loader.
func NewLoader() *Loader {
	return &Loader{}
}

// Load reads the config file at the given path and returns a populated Config.
// The file is parsed line by line, ignoring empty lines and comments (#).
// Each key=value line sets the corresponding field in Config.
func (c *Loader) Load(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cfg := &Config{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "grok_api_key":
			cfg.GrokAPIKey = value
		case "openai_api_key":
			cfg.OpenAIAPIKey = value
		case "openrouter_api_key":
			cfg.OpenRouterAPIKey = value
		case "grok_model":
			cfg.GrokModel = value
		case "openai_model":
			cfg.OpenAIModel = value
		case "openrouter_model":
			cfg.OpenRouterModel = value
		case "lm_proxy_api_key":
			cfg.LMProxyAPIKey = value
		case "lm_proxy_url":
			cfg.LMProxyURL = value
		case "lm_proxy_model":
			cfg.LMProxyModel = value
		case "default_provider":
			cfg.DefaultProvider = value
		case "system_prompt":
			cfg.SystemPrompt = value
		}
	}
	return cfg, scanner.Err()
}

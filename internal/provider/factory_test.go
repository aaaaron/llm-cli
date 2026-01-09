package provider

import (
	"testing"

	"llm/internal/config"
)

func TestFactory_Create(t *testing.T) {
	tests := []struct {
		model     string
		cfg       *config.Config
		wantModel string
	}{
		{"openrouter", &config.Config{OpenRouterAPIKey: "key", OpenRouterModel: "model"}, "model"},
		{"openrouter", &config.Config{OpenRouterAPIKey: "key"}, "openai/gpt-3.5-turbo"},
		{"grok", &config.Config{GrokAPIKey: "key", GrokModel: "model"}, "model"},
		{"grok", &config.Config{GrokAPIKey: "key"}, "grok-3"},
		{"openai", &config.Config{OpenAIAPIKey: "key", OpenAIModel: "model"}, "model"},
		{"openai", &config.Config{OpenAIAPIKey: "key"}, "gpt-3.5-turbo"},
		{"invalid", &config.Config{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			factory := NewFactory()
			prov, model, err := factory.Create(tt.model, tt.cfg)
			if tt.model == "invalid" {
				if err == nil {
					t.Error("expected error for invalid model")
				}
				return
			}
			if err != nil {
				t.Errorf("Create() error = %v", err)
				return
			}
			if model != tt.wantModel {
				t.Errorf("Create() model = %v, want %v", model, tt.wantModel)
			}
			if prov == nil {
				t.Error("Create() provider = nil")
			}
		})
	}
}

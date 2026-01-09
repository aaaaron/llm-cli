package config

import (
	"os"
	"testing"
)

func TestLoader_Load(t *testing.T) {
	// Happy path
	content := `grok_api_key=test_grok
openai_api_key=test_openai
grok_model=grok-3
openai_model=gpt-3.5-turbo
default_provider=openrouter
system_prompt=Test prompt
`
	tmpFile, err := os.CreateTemp("", "config_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	loader := NewLoader()
	cfg, err := loader.Load(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	if cfg.GrokAPIKey != "test_grok" {
		t.Errorf("Expected GrokAPIKey 'test_grok', got '%s'", cfg.GrokAPIKey)
	}
	if cfg.OpenAIAPIKey != "test_openai" {
		t.Errorf("Expected OpenAIAPIKey 'test_openai', got '%s'", cfg.OpenAIAPIKey)
	}
	if cfg.GrokModel != "grok-3" {
		t.Errorf("Expected GrokModel 'grok-3', got '%s'", cfg.GrokModel)
	}
	if cfg.OpenAIModel != "gpt-3.5-turbo" {
		t.Errorf("Expected OpenAIModel 'gpt-3.5-turbo', got '%s'", cfg.OpenAIModel)
	}
	if cfg.DefaultProvider != "openrouter" {
		t.Errorf("Expected DefaultProvider 'openrouter', got '%s'", cfg.DefaultProvider)
	}
	if cfg.SystemPrompt != "Test prompt" {
		t.Errorf("Expected SystemPrompt 'Test prompt', got '%s'", cfg.SystemPrompt)
	}
}

func TestLoader_InvalidLines(t *testing.T) {
	content := `# comment
invalid line
key without= value
 = bad
default_provider = openrouter
`
	tmpFile, err := os.CreateTemp("", "config_invalid")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	loader := NewLoader()
	cfg, err := loader.Load(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	if cfg.DefaultProvider != "openrouter" {
		t.Errorf("DefaultProvider = %q, want openrouter", cfg.DefaultProvider)
	}
}

func TestLoader_EmptyFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config_empty")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.Close()

	loader := NewLoader()
	cfg, err := loader.Load(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	if cfg.SystemPrompt != "" {
		t.Errorf("Expected empty SystemPrompt, got %q", cfg.SystemPrompt)
	}
}

func TestLoader_MissingFile(t *testing.T) {
	loader := NewLoader()
	_, err := loader.Load("/nonexistent/llmrc")
	if err == nil {
		t.Error("Expected error for missing file")
	}
	if !os.IsNotExist(err) {
		t.Errorf("Expected IsNotExist error, got %v", err)
	}
}

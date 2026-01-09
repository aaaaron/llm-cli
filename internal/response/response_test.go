package response

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"llm/internal/session"
)

func TestHandle_WithCodeBlocks(t *testing.T) {
	// Create a temp directory to work in
	tmpDir, err := os.MkdirTemp("", "response_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp dir for file creation
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Mock provider response with code blocks
	mockResponse := `Here's a Python script that prints hello world:

` + "```python" + `
print("Hello, World!")
` + "```" + `

And here's a Go version:

` + "```go" + `
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
` + "```" + `

Hope that helps!`

	manager := session.NewManager()
	sess := manager.NewSession("test")

	// Handle should save files from code blocks
	Handle(manager, sess, "write hello world", mockResponse, "plain")

	// Check that generated files were created
	if _, err := os.Stat(filepath.Join(tmpDir, "generated_1.py")); os.IsNotExist(err) {
		t.Error("Expected generated_1.py to be created")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "generated_2.go")); os.IsNotExist(err) {
		t.Error("Expected generated_2.go to be created")
	}

	// Verify content
	pyContent, _ := os.ReadFile(filepath.Join(tmpDir, "generated_1.py"))
	if !strings.Contains(string(pyContent), "Hello, World!") {
		t.Errorf("Python file content incorrect: %s", pyContent)
	}

	goContent, _ := os.ReadFile(filepath.Join(tmpDir, "generated_2.go"))
	if !strings.Contains(string(goContent), "Hello, World!") {
		t.Errorf("Go file content incorrect: %s", goContent)
	}
}

func TestHandle_SkipsBashBlocks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "response_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Response with only bash block - should not create file
	mockResponse := `Run this command:

` + "```bash" + `
echo "hello"
` + "```"

	manager := session.NewManager()
	sess := manager.NewSession("test")

	Handle(manager, sess, "run command", mockResponse, "plain")

	// No files should be created for bash blocks
	files, _ := filepath.Glob(filepath.Join(tmpDir, "generated_*"))
	if len(files) != 0 {
		t.Errorf("Expected no generated files for bash blocks, got %d", len(files))
	}
}

func TestSaveFilesFromResponse_MultipleLanguages(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "response_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	mockResponse := "```javascript\nconsole.log('js');\n```\n" +
		"```typescript\nconsole.log('ts');\n```\n" +
		"```rust\nfn main() {}\n```\n" +
		"```json\n{\"key\": \"value\"}\n```"

	saveFilesFromResponse(mockResponse)

	// Check extensions are mapped correctly
	expected := map[string]string{
		"generated_1.js":   "console.log('js');",
		"generated_2.ts":   "console.log('ts');",
		"generated_3.rs":   "fn main() {}",
		"generated_4.json": `{"key": "value"}`,
	}

	for filename, expectedContent := range expected {
		content, err := os.ReadFile(filepath.Join(tmpDir, filename))
		if err != nil {
			t.Errorf("Failed to read %s: %v", filename, err)
			continue
		}
		if strings.TrimSpace(string(content)) != expectedContent {
			t.Errorf("%s content = %q, want %q", filename, strings.TrimSpace(string(content)), expectedContent)
		}
	}
}

func TestSaveFilesFromResponse_UnknownLanguage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "response_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Unknown language should use the language as extension
	mockResponse := "```xyz\nsome content\n```"

	saveFilesFromResponse(mockResponse)

	if _, err := os.Stat(filepath.Join(tmpDir, "generated_1.xyz")); os.IsNotExist(err) {
		t.Error("Expected generated_1.xyz for unknown language")
	}
}

func TestSaveFilesFromResponse_NoLanguage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "response_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// No language specified should default to .txt
	mockResponse := "```\nplain text content\n```"

	saveFilesFromResponse(mockResponse)

	if _, err := os.Stat(filepath.Join(tmpDir, "generated_1.txt")); os.IsNotExist(err) {
		t.Error("Expected generated_1.txt for no language specified")
	}
}

func TestSaveFilesFromResponse_NoCodeBlocks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "response_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	mockResponse := "This is just plain text without any code blocks."

	saveFilesFromResponse(mockResponse)

	files, _ := filepath.Glob(filepath.Join(tmpDir, "generated_*"))
	if len(files) != 0 {
		t.Errorf("Expected no files for response without code blocks, got %d", len(files))
	}
}

func TestLanguageExtensions(t *testing.T) {
	// Verify key language mappings
	tests := []struct {
		lang string
		ext  string
	}{
		{"python", "py"},
		{"javascript", "js"},
		{"typescript", "ts"},
		{"go", "go"},
		{"rust", "rs"},
		{"ruby", "rb"},
		{"c++", "cpp"},
		{"cpp", "cpp"},
		{"kotlin", "kt"},
		{"swift", "swift"},
		{"haskell", "hs"},
	}

	for _, tt := range tests {
		if got := languageExtensions[tt.lang]; got != tt.ext {
			t.Errorf("languageExtensions[%q] = %q, want %q", tt.lang, got, tt.ext)
		}
	}
}

func TestIsUserInteractive(t *testing.T) {
	// In test environment, stdout is typically not a terminal
	// This is a basic sanity check that the function runs
	result := IsUserInteractive()
	// We can't assert the value since it depends on the test environment
	// but we verify the function doesn't panic
	_ = result
}

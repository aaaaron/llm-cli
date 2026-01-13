# Code Quality Issues - LLM CLI

Generated: 2026-01-13
Total Issues: 32

---

## 🔴 CRITICAL PRIORITY (3 issues)

### Issue #1: Unsafe type assertion causes potential panic
**Priority:** Critical
**Labels:** bug, priority:critical
**Location:** `cmd/llm/main.go:303`

**Problem:**
Unsafe type assertion will cause panic if provider isn't OpenRouterProvider.

```go
prov.(*provider.OpenRouterProvider).FreeMode = true
```

**Impact:**
- Application crashes if internal logic changes
- No graceful error handling

**Fix:**
```go
if orProv, ok := prov.(*provider.OpenRouterProvider); ok {
    orProv.FreeMode = true
} else {
    return fmt.Errorf("free mode only supported for OpenRouter provider")
}
```

---

### Issue #2: Command injection security risk
**Priority:** Critical
**Labels:** security, priority:critical
**Location:** `internal/response/response.go:206`

**Problem:**
Executes arbitrary shell commands from LLM responses with minimal protection.

```go
command := exec.Command("sh", "-c", cmd)
```

**Security Risk:**
- LLM responses could craft malicious commands
- User confirmation exists but doesn't prevent shell tricks (comments, semicolons, etc.)
- No sanitization or sandboxing

**Example Attack:**
```bash
ls -la  # innocent comment; rm -rf / --no-preserve-root
```

**Recommended Fixes:**
1. Parse and validate commands (no chaining, redirects, or subshells)
2. Use allowlist of safe commands
3. Run in restricted environment (containers, chroot, firejail)
4. Show full command with expansions before execution
5. Add dry-run mode
6. Implement command timeout

---

### Issue #3: Silent error handling in session file path
**Priority:** Critical
**Labels:** bug, priority:critical
**Location:** `internal/session/session.go:38-39`

**Problem:**
If `UserHomeDir()` fails, session file path becomes invalid.

```go
home, _ := os.UserHomeDir()
return filepath.Join(home, ".llm_session.json")
```

**Impact:**
- Session file created in wrong location (`.llm_session.json` in current dir)
- Silently ignores environment issues
- Data loss or corruption risk

**Fix:**
```go
func (m *Manager) sessionFile() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return "", fmt.Errorf("failed to get home directory: %w", err)
    }
    return filepath.Join(home, ".llm_session.json"), nil
}
```

Update all callers to handle error.

---

## 🟡 CODE SMELLS (10 issues)

### Issue #4: Massive code duplication in SSE stream parsing
**Priority:** High
**Labels:** refactoring, code-smell, technical-debt
**Locations:**
- `internal/provider/provider.go:234-263` (GrokProvider)
- `internal/provider/provider.go:329-358` (LMProxyProvider)
- `internal/provider/provider.go:426-455` (OpenRouterProvider)

**Problem:**
Nearly identical SSE parsing logic repeated 3 times (~90 lines duplicated).

**Impact:**
- Hard to maintain (bugs need fixing in 3 places)
- Violates DRY principle
- Increased codebase size

**Fix:**
Extract common SSE parsing into shared function:

```go
func parseSSEStream(reader *bufio.Reader, onChunk func(string)) (string, error) {
    var fullResponse string
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
```

---

### Issue #5: Inefficient string concatenation in streaming
**Priority:** High
**Labels:** performance, code-smell
**Locations:**
- `internal/provider/provider.go:170, 257, 352, 449`
- `cmd/llm/main.go:323`

**Problem:**
String concatenation with `+=` in loops creates new strings each time.

```go
fullResponse += chunk  // Inefficient
accumulated += chunk   // Inefficient
```

**Impact:**
- Excessive memory allocation for long responses
- O(n²) complexity for string building
- Poor performance with large streaming responses

**Fix:**
Use `strings.Builder`:

```go
var builder strings.Builder
for chunk := range chunks {
    builder.WriteString(chunk)
}
fullResponse := builder.String()
```

---

### Issue #6: God object anti-pattern in main.go
**Priority:** Medium
**Labels:** refactoring, code-smell, maintainability
**Location:** `cmd/llm/main.go`

**Problem:**
Main function does everything (344 lines):
- Flag parsing
- Config loading
- Provider creation
- Session management
- Signal handling
- Model discovery
- API queries
- Response processing

**Impact:**
- Hard to test
- Poor separation of concerns
- Difficult to understand and maintain

**Fix:**
Extract into focused functions or CLI application struct:

```go
type CLI struct {
    config   *config.Config
    session  *session.Manager
    provider provider.Provider
    flags    *Flags
}

func (c *CLI) Run() error {
    if err := c.loadConfig(); err != nil {
        return err
    }
    if err := c.setupProvider(); err != nil {
        return err
    }
    // ... etc
}
```

---

### Issue #7: Unnecessary abstraction - Processor interface
**Priority:** Low
**Labels:** refactoring, over-engineering, YAGNI
**Location:** `internal/response/processor.go`

**Problem:**
Interface with single implementation is over-engineering.

```go
type Processor interface {
    Process(...) error
}

type DefaultProcessor struct{}  // Only implementation
```

**Impact:**
- Unnecessary complexity
- YAGNI violation
- Harder to navigate code

**Fix:**
Remove interface, use `Handle()` function directly in main.go.

---

### Issue #8: Unused function parameter
**Priority:** Low
**Labels:** code-smell, cleanup
**Location:** `internal/response/processor.go:21`

**Problem:**
The `query` parameter is passed but never used.

```go
func (p *DefaultProcessor) Process(sessionManager *session.Manager,
    sess *session.Session, query, response, format string) error {
    Handle(sessionManager, sess, query, response, format)  // 'query' unused in Handle
    return nil
}
```

**Fix:**
Remove unused parameter or use it for logging/debugging.

---

### Issue #9: HTTP client created on every request
**Priority:** Medium
**Labels:** performance, resource-management
**Locations:**
- `internal/provider/provider.go:221, 316, 413`

**Problem:**
Creates new HTTP client for every request instead of reusing.

```go
client := &http.Client{}
resp, err := client.Do(req)
```

**Impact:**
- Inefficient connection pooling
- Higher latency
- More resource usage

**Fix:**
Reuse HTTP client:

```go
// In provider struct
type GrokProvider struct {
    apiKey string
    client *http.Client
}

// Initialize once
func NewGrokProvider(apiKey string) *GrokProvider {
    return &GrokProvider{
        apiKey: apiKey,
        client: &http.Client{Timeout: StreamingRequestTimeout},
    }
}
```

---

### Issue #10: Magic numbers throughout codebase
**Priority:** Low
**Labels:** code-smell, maintainability
**Locations:** Multiple files

**Problem:**
Magic numbers without descriptive names:

```go
24*time.Hour              // session.go:74 - session expiration
120 * time.Second         // provider.go:27 - streaming timeout
30 * time.Second          // provider.go:29 - model discovery timeout
8000                      // provider.go:276 - default proxy port
0644                      // Multiple files - file permissions
```

**Fix:**
Define constants:

```go
const (
    SessionExpirationDuration = 24 * time.Hour
    StreamingRequestTimeout   = 120 * time.Second
    ModelDiscoveryTimeout     = 30 * time.Second
    DefaultProxyPort         = 8000
    DefaultFilePermissions   = 0644
)
```

---

### Issue #11: Inconsistent error handling with os.Exit
**Priority:** High
**Labels:** testing, code-smell, maintainability
**Location:** `cmd/llm/main.go` (15 occurrences)

**Problem:**
15 `os.Exit()` calls in main.go instead of proper error propagation.

**Locations:**
Lines 111, 159, 179, 187, 238, 247, 256, 267, 277, 294, 329

**Impact:**
- Impossible to test
- Deferred cleanup doesn't run
- Can't handle errors gracefully

**Fix:**
Return errors from main logic:

```go
func run() error {
    // ... all logic here
    if err != nil {
        return fmt.Errorf("failed to load config: %w", err)
    }
    return nil
}

func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

---

### Issue #12: Multiple ignored errors
**Priority:** Medium
**Labels:** bug, error-handling
**Locations:**
- `cmd/llm/main.go:298, 337`
- `internal/response/response.go:171, 273`
- `internal/config/config.go:86`

**Problem:**
Errors logged but not handled:

```go
if err := sessionManager.Save(sess); err != nil {
    fmt.Fprintf(os.Stderr, "Error saving session: %v\n", err)
    // continues anyway
}
```

**Impact:**
- Data loss risk
- Silent failures
- Difficult debugging

**Fix:**
Propagate errors or take corrective action.

---

### Issue #13: Poor variable naming
**Priority:** Low
**Labels:** code-style, readability
**Locations:** Throughout codebase

**Problem:**
Abbreviated variable names reduce readability:

```go
prov   // should be: provider
cfg    // should be: config
sess   // should be: session
```

**Impact:**
- Reduced code readability
- Harder for new developers
- Inconsistent with Go conventions

**Fix:**
Use full descriptive names (Go encourages clarity over brevity).

---

## 🟠 POOR PRACTICES (9 issues)

### Issue #14: Missing input validation
**Priority:** High
**Labels:** validation, reliability
**Locations:** Multiple

**Problems:**
1. No check if API keys are empty before requests
2. No validation of model names
3. No limits on message history size
4. No timeout on user input prompts

**Impact:**
- Confusing error messages
- Potential API cost explosions
- Memory issues with large sessions

**Fix:**
Add validation layer:

```go
func validateConfig(cfg *config.Config, provider string) error {
    switch provider {
    case "openai":
        if cfg.OpenAIAPIKey == "" {
            return errors.New("OpenAI API key required")
        }
    // ... etc
    }
    return nil
}

func validateSession(sess *session.Session) error {
    const maxMessages = 100
    if len(sess.Messages) > maxMessages {
        return fmt.Errorf("session too large (%d messages, max %d)",
            len(sess.Messages), maxMessages)
    }
    return nil
}
```

---

### Issue #15: No resource cleanup in error paths
**Priority:** Medium
**Labels:** bug, resource-management
**Locations:**
- `internal/provider/provider.go:249, 344, 441`

**Problem:**
If unmarshaling fails during streaming, code continues without checking connection health.

```go
var event map[string]interface{}
if err := json.Unmarshal([]byte(data), &event); err != nil {
    continue  // Silently skips, connection might be broken
}
```

**Impact:**
- Could loop infinitely on malformed responses
- Wastes resources
- Poor error detection

**Fix:**
Track consecutive errors and abort:

```go
consecutiveErrors := 0
const maxConsecutiveErrors = 10

if err := json.Unmarshal([]byte(data), &event); err != nil {
    consecutiveErrors++
    if consecutiveErrors > maxConsecutiveErrors {
        return "", fmt.Errorf("too many parse errors: %w", err)
    }
    continue
}
consecutiveErrors = 0
```

---

### Issue #16: No rate limiting or backoff
**Priority:** Medium
**Labels:** enhancement, reliability
**Locations:** All provider implementations

**Problem:**
Tool can hammer APIs without backoff or retry logic.

**Impact:**
- Hit rate limits easily
- Poor user experience on transient failures
- Potential API key suspension

**Fix:**
Implement exponential backoff:

```go
import "github.com/cenkalti/backoff/v4"

func (p *OpenAIProvider) QueryWithRetry(...) (string, error) {
    operation := func() (string, error) {
        return p.Query(...)
    }

    bo := backoff.NewExponentialBackOff()
    return backoff.Retry(operation, bo)
}
```

---

### Issue #17: No request size validation
**Priority:** Medium
**Labels:** cost, reliability
**Location:** All providers

**Problem:**
Could accumulate massive session history and send it all to APIs.

**Impact:**
- Unexpected API costs
- Request timeouts
- Poor performance

**Fix:**
Add size limits:

```go
func estimateTokens(messages []session.Message) int {
    // Rough estimate: 4 chars ≈ 1 token
    total := 0
    for _, msg := range messages {
        total += len(msg.Content) / 4
    }
    return total
}

func validateRequestSize(messages []session.Message) error {
    const maxTokens = 10000
    tokens := estimateTokens(messages)
    if tokens > maxTokens {
        return fmt.Errorf("request too large: ~%d tokens (max %d)",
            tokens, maxTokens)
    }
    return nil
}
```

---

### Issue #18: Potential race condition in signal handler
**Priority:** Medium
**Labels:** concurrency, bug
**Location:** `cmd/llm/main.go:313-318`

**Problem:**
Signal handler goroutine could race with main goroutine's defer.

```go
defer s.Stop()  // Main goroutine

go func() {
    <-sigChan
    s.Stop()  // Signal goroutine - race!
    fmt.Fprintln(os.Stderr, "\nInterrupted by user")
    os.Exit(130)
}()
```

**Impact:**
- Double-stop could cause issues
- Race detector would flag this

**Fix:**
Use sync.Once or context:

```go
var stopOnce sync.Once
defer func() {
    stopOnce.Do(func() { s.Stop() })
}()

go func() {
    <-sigChan
    stopOnce.Do(func() { s.Stop() })
    fmt.Fprintln(os.Stderr, "\nInterrupted by user")
    os.Exit(130)
}()
```

---

### Issue #19: Hardcoded file paths
**Priority:** Low
**Labels:** enhancement, configurability
**Locations:**
- Session file: `~/.llm_session.json`
- Config file: `~/.llmrc`
- History file: `~/.bash_history` or `~/.zsh_history`

**Problem:**
All paths hardcoded to home directory.

**Impact:**
- No multi-user support
- No custom locations
- Hard to test

**Fix:**
Add environment variables:

```go
func (m *Manager) sessionFile() string {
    if path := os.Getenv("LLM_SESSION_FILE"); path != "" {
        return path
    }
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".llm_session.json")
}
```

---

### Issue #20: OpenRouter model discovery on every new session
**Priority:** Medium
**Labels:** performance, optimization
**Location:** `cmd/llm/main.go:249-300`

**Problem:**
When `_free_` is set but session doesn't have a model, queries API.

**Impact:**
- API call on every new session
- Could hit rate limits
- Slow startup

**Fix:**
Cache discovered model globally:

```go
var (
    cachedFreeModel string
    cacheExpiry     time.Time
    cacheMutex      sync.RWMutex
)

func getCachedOrDiscoverFreeModel(...) (string, error) {
    cacheMutex.RLock()
    if time.Now().Before(cacheExpiry) && cachedFreeModel != "" {
        model := cachedFreeModel
        cacheMutex.RUnlock()
        return model, nil
    }
    cacheMutex.RUnlock()

    // Discover and cache
    model := discoverFreeModel(...)
    cacheMutex.Lock()
    cachedFreeModel = model
    cacheExpiry = time.Now().Add(24 * time.Hour)
    cacheMutex.Unlock()

    return model, nil
}
```

---

### Issue #21: Inconsistent string comparison
**Priority:** Low
**Labels:** code-style, enhancement
**Location:** `internal/response/response.go:128`

**Problem:**
Verbose yes/no check:

```go
answer = strings.TrimSpace(strings.ToLower(answer))
if answer == "y" || answer == "yes" {
```

**Fix:**
Use switch or helper:

```go
func isYes(answer string) bool {
    answer = strings.TrimSpace(strings.ToLower(answer))
    return answer == "y" || answer == "yes"
}
```

---

### Issue #22: Terminal width fallback too large
**Priority:** Low
**Labels:** ux, enhancement
**Location:** `internal/response/response.go:104`

**Problem:**
80-column fallback might be too wide for some terminals.

```go
if err != nil || width < 10 {
    width = 80
}
```

**Fix:**
Use more conservative default:

```go
if err != nil || width < 10 {
    width = 70  // More universally safe
}
```

---

## 🔵 MINOR IMPROVEMENTS (5 issues)

### Issue #23: No logging framework
**Priority:** Low
**Labels:** enhancement, observability
**Locations:** Throughout codebase

**Problem:**
All output uses fmt.Fprintf/fmt.Printf.

**Impact:**
- No structured logging
- No log levels
- Hard to debug production issues

**Fix:**
Add logging library:

```go
import "log/slog"

logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
logger.Info("starting query", "provider", provider, "model", model)
logger.Error("query failed", "error", err)
```

---

### Issue #24: Test coverage unknown
**Priority:** Medium
**Labels:** testing, quality

**Problem:**
Test files exist but coverage couldn't be verified due to Go 1.25 download issue.

**Fix:**
1. Run tests: `go test ./... -cover`
2. Generate coverage report: `go test ./... -coverprofile=coverage.out`
3. Aim for >80% coverage
4. Add CI/CD to enforce coverage thresholds

---

### Issue #25: Factory pattern provides little value
**Priority:** Low
**Labels:** refactoring, simplification
**Location:** `internal/provider/factory.go`

**Problem:**
Factory has no state, could be a function.

```go
type Factory struct{}

func NewFactory() *Factory {
    return &Factory{}
}
```

**Fix:**
Replace with simple function:

```go
func CreateProvider(model string, cfg *config.Config) (Provider, string, error) {
    // ... implementation
}
```

---

### Issue #26: No graceful degradation
**Priority:** Low
**Labels:** enhancement, ux

**Problem:**
If Glamour markdown rendering fails, no fallback.

**Location:** `internal/ui/ui.go:42-46`

```go
r, _ := glamour.NewTermRenderer(...)
output, _ = r.Render(displayResponse)
```

**Fix:**
Add fallback to plain text:

```go
r, err := glamour.NewTermRenderer(...)
if err != nil {
    output = displayResponse  // Fallback to plain
} else {
    output, err = r.Render(displayResponse)
    if err != nil {
        output = displayResponse
    }
}
```

---

### Issue #27: No version compatibility checking
**Priority:** Low
**Labels:** enhancement, compatibility

**Problem:**
Session file has no version field.

**Impact:**
- Can't handle format changes
- Breaking changes require manual deletion

**Fix:**
Add version to session:

```go
type Session struct {
    Version             int       `json:"version"`
    Messages            []Message `json:"messages"`
    Provider            string    `json:"provider"`
    OpenRouterFreeModel string    `json:"openrouter_free_model,omitempty"`
}

const CurrentSessionVersion = 1

func (m *Manager) Load() (*Session, error) {
    // ... load file
    if sess.Version != CurrentSessionVersion {
        return m.migrate(sess)
    }
    return sess, nil
}
```

---

## Additional Recommendations

### Issue #28: Add .gitignore for generated files
**Priority:** Low
**Labels:** repository, housekeeping

**Problem:**
No .gitignore for `generated_*.ext` files created by tool.

**Fix:**
Create .gitignore:
```
generated_*
.llm_session.json
coverage.out
*.test
```

---

### Issue #29: No contribution guidelines
**Priority:** Low
**Labels:** documentation, community

**Fix:**
Add CONTRIBUTING.md with:
- Code style guide
- How to run tests
- PR process
- Issue templates

---

### Issue #30: Missing CI/CD pipeline
**Priority:** Medium
**Labels:** devops, quality

**Fix:**
Add GitHub Actions workflow:
- Run tests on PR
- Check code coverage
- Run linters (golangci-lint)
- Build binaries
- Security scanning

---

### Issue #31: No changelog
**Priority:** Low
**Labels:** documentation

**Fix:**
Add CHANGELOG.md following Keep a Changelog format.

---

### Issue #32: Environment variable support for API keys
**Priority:** Medium
**Labels:** enhancement, security
**Location:** `internal/config/config.go`

**Problem:**
Only reads API keys from config file.

**Impact:**
- Less secure (keys in file)
- Harder to use in CI/CD
- No 12-factor app compliance

**Fix:**
Check environment variables first:

```go
func (c *Loader) Load(path string) (*Config, error) {
    cfg, err := c.loadFromFile(path)
    if err != nil {
        cfg = &Config{}
    }

    // Environment variables override file
    if key := os.Getenv("OPENAI_API_KEY"); key != "" {
        cfg.OpenAIAPIKey = key
    }
    if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
        cfg.OpenRouterAPIKey = key
    }
    // ... etc

    return cfg, nil
}
```

---

## Summary

- **Critical:** 3 issues - Fix immediately
- **High Priority:** 6 issues - Fix soon
- **Medium Priority:** 10 issues - Plan for next release
- **Low Priority:** 13 issues - Technical debt backlog

**Suggested Fix Order:**
1. Issue #1 (Type assertion panic)
2. Issue #3 (Session file error)
3. Issue #2 (Command injection - needs design discussion)
4. Issue #4 (Code duplication)
5. Issue #5 (String concatenation)
6. Issue #11 (Error handling with os.Exit)
7. Issues #14-20 (Poor practices)
8. Remaining issues as time permits

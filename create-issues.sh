#!/bin/bash
# Script to create GitHub issues from code analysis
# Requires: gh CLI (GitHub CLI) - install from https://cli.github.com/

set -e

# Check if gh is installed
if ! command -v gh &> /dev/null; then
    echo "Error: gh CLI is not installed"
    echo "Install from: https://cli.github.com/"
    exit 1
fi

# Check if authenticated
if ! gh auth status &> /dev/null; then
    echo "Error: Not authenticated with GitHub"
    echo "Run: gh auth login"
    exit 1
fi

echo "Creating 32 GitHub issues from code analysis..."
echo

# Critical Issues
echo "Creating critical issues (1-3)..."

gh issue create \
  --title "Critical: Unsafe type assertion causes potential panic" \
  --label "bug,priority:critical" \
  --body "## Problem
Unsafe type assertion in main.go:303 will cause panic if provider isn't OpenRouterProvider.

**Location:** \`cmd/llm/main.go:303\`

\`\`\`go
prov.(*provider.OpenRouterProvider).FreeMode = true
\`\`\`

## Impact
- Application crashes if internal logic changes
- No graceful error handling

## Fix
Use type checking with ok pattern:
\`\`\`go
if orProv, ok := prov.(*provider.OpenRouterProvider); ok {
    orProv.FreeMode = true
} else {
    return fmt.Errorf(\"free mode only supported for OpenRouter provider\")
}
\`\`\`

## Priority
🔴 **CRITICAL** - Fix immediately"

gh issue create \
  --title "Critical: Command injection security risk" \
  --label "security,priority:critical" \
  --body "## Problem
Executes arbitrary shell commands from LLM responses with minimal protection.

**Location:** \`internal/response/response.go:206\`

\`\`\`go
command := exec.Command(\"sh\", \"-c\", cmd)
\`\`\`

## Security Risk
- LLM responses could craft malicious commands
- User confirmation exists but doesn't prevent shell tricks
- No sanitization or sandboxing

## Example Attack
\`\`\`bash
ls -la  # innocent comment; rm -rf / --no-preserve-root
\`\`\`

## Recommended Fixes
1. Parse and validate commands (no chaining, redirects, or subshells)
2. Use allowlist of safe commands
3. Run in restricted environment (containers, chroot)
4. Show full command with expansions before execution
5. Add dry-run mode

## Priority
🔴 **CRITICAL** - Security vulnerability"

gh issue create \
  --title "Critical: Silent error handling in session file path" \
  --label "bug,priority:critical" \
  --body "## Problem
If \`UserHomeDir()\` fails, session file path becomes invalid.

**Location:** \`internal/session/session.go:38-39\`

\`\`\`go
home, _ := os.UserHomeDir()
return filepath.Join(home, \".llm_session.json\")
\`\`\`

## Impact
- Session file created in wrong location
- Silently ignores environment issues
- Data loss or corruption risk

## Fix
Handle error properly:
\`\`\`go
func (m *Manager) sessionFile() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return \"\", fmt.Errorf(\"failed to get home directory: %w\", err)
    }
    return filepath.Join(home, \".llm_session.json\"), nil
}
\`\`\`

## Priority
🔴 **CRITICAL** - Data integrity issue"

echo "Created 3 critical issues"
echo

# Code Smells
echo "Creating code smell issues (4-13)..."

gh issue create \
  --title "Massive code duplication in SSE stream parsing" \
  --label "refactoring,code-smell,technical-debt,priority:high" \
  --body "## Problem
Nearly identical SSE parsing logic repeated 3 times (~90 lines duplicated).

**Locations:**
- \`internal/provider/provider.go:234-263\` (GrokProvider)
- \`internal/provider/provider.go:329-358\` (LMProxyProvider)
- \`internal/provider/provider.go:426-455\` (OpenRouterProvider)

## Impact
- Hard to maintain (bugs need fixing in 3 places)
- Violates DRY principle
- Increased codebase size

## Fix
Extract common SSE parsing into shared function. See ISSUES.md for implementation."

gh issue create \
  --title "Inefficient string concatenation in streaming" \
  --label "performance,code-smell,priority:high" \
  --body "## Problem
String concatenation with \`+=\` in loops creates new strings each time.

**Locations:**
- \`internal/provider/provider.go:170, 257, 352, 449\`
- \`cmd/llm/main.go:323\`

\`\`\`go
fullResponse += chunk  // Inefficient
accumulated += chunk   // Inefficient
\`\`\`

## Impact
- Excessive memory allocation for long responses
- O(n²) complexity
- Poor performance with large streaming responses

## Fix
Use \`strings.Builder\`"

gh issue create \
  --title "God object anti-pattern in main.go" \
  --label "refactoring,code-smell,maintainability" \
  --body "## Problem
Main function does everything (344 lines):
- Flag parsing
- Config loading
- Provider creation
- Session management
- Signal handling
- Model discovery
- API queries
- Response processing

**Location:** \`cmd/llm/main.go\`

## Impact
- Hard to test
- Poor separation of concerns
- Difficult to understand and maintain

## Fix
Extract into focused functions or CLI application struct. See ISSUES.md for example."

gh issue create \
  --title "Unnecessary abstraction - Processor interface" \
  --label "refactoring,over-engineering,YAGNI" \
  --body "## Problem
Interface with single implementation is over-engineering.

**Location:** \`internal/response/processor.go\`

## Impact
- Unnecessary complexity
- YAGNI violation
- Harder to navigate code

## Fix
Remove interface, use \`Handle()\` function directly."

gh issue create \
  --title "Unused function parameter" \
  --label "code-smell,cleanup" \
  --body "## Problem
The \`query\` parameter in Process() is passed but never used.

**Location:** \`internal/response/processor.go:21\`

## Fix
Remove unused parameter or use it for logging."

gh issue create \
  --title "HTTP client created on every request" \
  --label "performance,resource-management" \
  --body "## Problem
Creates new HTTP client for every request instead of reusing.

**Locations:**
- \`internal/provider/provider.go:221, 316, 413\`

## Impact
- Inefficient connection pooling
- Higher latency
- More resource usage

## Fix
Store HTTP client in provider struct and reuse. See ISSUES.md for implementation."

gh issue create \
  --title "Magic numbers throughout codebase" \
  --label "code-smell,maintainability" \
  --body "## Problem
Magic numbers without descriptive names:
- \`24*time.Hour\` - session expiration
- \`120 * time.Second\` - streaming timeout
- \`30 * time.Second\` - model discovery timeout
- \`8000\` - default proxy port
- \`0644\` - file permissions

## Fix
Define constants with descriptive names."

gh issue create \
  --title "Inconsistent error handling with os.Exit" \
  --label "testing,code-smell,maintainability,priority:high" \
  --body "## Problem
15 \`os.Exit()\` calls in main.go instead of proper error propagation.

**Location:** \`cmd/llm/main.go\` (lines 111, 159, 179, 187, 238, 247, 256, 267, 277, 294, 329)

## Impact
- Impossible to test
- Deferred cleanup doesn't run
- Can't handle errors gracefully

## Fix
Return errors from main logic. See ISSUES.md for implementation."

gh issue create \
  --title "Multiple ignored errors" \
  --label "bug,error-handling" \
  --body "## Problem
Errors logged but not handled:

**Locations:**
- \`cmd/llm/main.go:298, 337\`
- \`internal/response/response.go:171, 273\`
- \`internal/config/config.go:86\`

## Impact
- Data loss risk
- Silent failures
- Difficult debugging

## Fix
Propagate errors or take corrective action."

gh issue create \
  --title "Poor variable naming" \
  --label "code-style,readability" \
  --body "## Problem
Abbreviated variable names reduce readability:
- \`prov\` should be \`provider\`
- \`cfg\` should be \`config\`
- \`sess\` should be \`session\`

## Impact
- Reduced code readability
- Harder for new developers
- Inconsistent with Go conventions

## Fix
Use full descriptive names."

echo "Created 10 code smell issues"
echo

# Poor Practices
echo "Creating poor practice issues (14-22)..."

gh issue create \
  --title "Missing input validation" \
  --label "validation,reliability,priority:high" \
  --body "## Problems
1. No check if API keys are empty before requests
2. No validation of model names
3. No limits on message history size
4. No timeout on user input prompts

## Impact
- Confusing error messages
- Potential API cost explosions
- Memory issues with large sessions

## Fix
Add validation layer. See ISSUES.md for implementation."

gh issue create \
  --title "No resource cleanup in error paths" \
  --label "bug,resource-management" \
  --body "## Problem
If unmarshaling fails during streaming, code continues without checking connection health.

**Locations:** \`internal/provider/provider.go:249, 344, 441\`

## Impact
- Could loop infinitely on malformed responses
- Wastes resources
- Poor error detection

## Fix
Track consecutive errors and abort. See ISSUES.md for implementation."

gh issue create \
  --title "No rate limiting or backoff" \
  --label "enhancement,reliability" \
  --body "## Problem
Tool can hammer APIs without backoff or retry logic.

## Impact
- Hit rate limits easily
- Poor user experience on transient failures
- Potential API key suspension

## Fix
Implement exponential backoff. See ISSUES.md for implementation."

gh issue create \
  --title "No request size validation" \
  --label "cost,reliability" \
  --body "## Problem
Could accumulate massive session history and send it all to APIs.

## Impact
- Unexpected API costs
- Request timeouts
- Poor performance

## Fix
Add size limits with token estimation. See ISSUES.md for implementation."

gh issue create \
  --title "Potential race condition in signal handler" \
  --label "concurrency,bug" \
  --body "## Problem
Signal handler goroutine could race with main goroutine's defer.

**Location:** \`cmd/llm/main.go:313-318\`

## Impact
- Double-stop could cause issues
- Race detector would flag this

## Fix
Use sync.Once. See ISSUES.md for implementation."

gh issue create \
  --title "Hardcoded file paths" \
  --label "enhancement,configurability" \
  --body "## Problem
All paths hardcoded to home directory:
- Session file: \`~/.llm_session.json\`
- Config file: \`~/.llmrc\`
- History file: \`~/.bash_history\` or \`~/.zsh_history\`

## Impact
- No multi-user support
- No custom locations
- Hard to test

## Fix
Add environment variable support. See ISSUES.md for implementation."

gh issue create \
  --title "OpenRouter model discovery on every new session" \
  --label "performance,optimization" \
  --body "## Problem
When \`_free_\` is set but session doesn't have a model, queries API every time.

**Location:** \`cmd/llm/main.go:249-300\`

## Impact
- API call on every new session
- Could hit rate limits
- Slow startup

## Fix
Cache discovered model globally with expiration. See ISSUES.md for implementation."

gh issue create \
  --title "Inconsistent string comparison" \
  --label "code-style,enhancement" \
  --body "## Problem
Verbose yes/no check:

**Location:** \`internal/response/response.go:128\`

\`\`\`go
answer = strings.TrimSpace(strings.ToLower(answer))
if answer == \"y\" || answer == \"yes\" {
\`\`\`

## Fix
Use helper function for clarity."

gh issue create \
  --title "Terminal width fallback too large" \
  --label "ux,enhancement" \
  --body "## Problem
80-column fallback might be too wide for some terminals.

**Location:** \`internal/response/response.go:104\`

## Fix
Use more conservative default (70 columns)."

echo "Created 9 poor practice issues"
echo

# Minor Improvements
echo "Creating minor improvement issues (23-32)..."

gh issue create \
  --title "No logging framework" \
  --label "enhancement,observability" \
  --body "## Problem
All output uses fmt.Fprintf/fmt.Printf.

## Impact
- No structured logging
- No log levels
- Hard to debug production issues

## Fix
Add logging library (log/slog). See ISSUES.md for implementation."

gh issue create \
  --title "Test coverage unknown" \
  --label "testing,quality,priority:medium" \
  --body "## Problem
Test files exist but coverage couldn't be verified.

## Fix
1. Run tests: \`go test ./... -cover\`
2. Generate coverage report: \`go test ./... -coverprofile=coverage.out\`
3. Aim for >80% coverage
4. Add CI/CD to enforce coverage thresholds"

gh issue create \
  --title "Factory pattern provides little value" \
  --label "refactoring,simplification" \
  --body "## Problem
Factory has no state, could be a function.

**Location:** \`internal/provider/factory.go\`

## Fix
Replace with simple function instead of struct."

gh issue create \
  --title "No graceful degradation for markdown rendering" \
  --label "enhancement,ux" \
  --body "## Problem
If Glamour markdown rendering fails, no fallback.

**Location:** \`internal/ui/ui.go:42-46\`

## Fix
Add fallback to plain text. See ISSUES.md for implementation."

gh issue create \
  --title "No version compatibility checking" \
  --label "enhancement,compatibility" \
  --body "## Problem
Session file has no version field.

## Impact
- Can't handle format changes
- Breaking changes require manual deletion

## Fix
Add version field to Session struct. See ISSUES.md for implementation."

gh issue create \
  --title "Add .gitignore for generated files" \
  --label "repository,housekeeping" \
  --body "## Problem
No .gitignore for \`generated_*.ext\` files created by tool.

## Fix
Create .gitignore:
\`\`\`
generated_*
.llm_session.json
coverage.out
*.test
\`\`\`"

gh issue create \
  --title "No contribution guidelines" \
  --label "documentation,community" \
  --body "## Fix
Add CONTRIBUTING.md with:
- Code style guide
- How to run tests
- PR process
- Issue templates"

gh issue create \
  --title "Missing CI/CD pipeline" \
  --label "devops,quality,priority:medium" \
  --body "## Fix
Add GitHub Actions workflow:
- Run tests on PR
- Check code coverage
- Run linters (golangci-lint)
- Build binaries
- Security scanning"

gh issue create \
  --title "No changelog" \
  --label "documentation" \
  --body "## Fix
Add CHANGELOG.md following Keep a Changelog format."

gh issue create \
  --title "Environment variable support for API keys" \
  --label "enhancement,security,priority:medium" \
  --body "## Problem
Only reads API keys from config file.

**Location:** \`internal/config/config.go\`

## Impact
- Less secure (keys in file)
- Harder to use in CI/CD
- No 12-factor app compliance

## Fix
Check environment variables first:
- \`OPENAI_API_KEY\`
- \`OPENROUTER_API_KEY\`
- \`GROK_API_KEY\`
- \`LM_PROXY_API_KEY\`

See ISSUES.md for implementation."

echo
echo "✅ Successfully created all 32 issues!"
echo
echo "View issues: gh issue list"
echo "Or visit: https://github.com/$(gh repo view --json nameWithOwner -q .nameWithOwner)/issues"

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/briandowns/spinner"

	"llm/internal/cli"
	"llm/internal/config"
	"llm/internal/provider"
	"llm/internal/response"
	"llm/internal/session"
)

const VERSION = "0.15"

var buildTime string

type openrouterModelsResponse struct {
	Data []openrouterModel `json:"data"`
}

type openrouterModel struct {
	ID      string `json:"id"`
	Pricing struct {
		Input  float64 `json:"input"`
		Output float64 `json:"output"`
	} `json:"pricing"`
	ContextLength int `json:"context_length"`
}

func verboseLog(enabled bool, format string, args ...any) {
	if !enabled {
		return
	}
	fmt.Printf("[verbose] "+format+"\n", args...)
}

func maskedValue(value string) string {
	if value == "" {
		return ""
	}
	return "***MASKED***"
}

// Select a provider based on which one appears to have the most parameters in its filename
func getModelParams(modelID string) float64 {
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)[bB]`)
	matches := re.FindStringSubmatch(modelID)
	if len(matches) > 1 {
		num, err := strconv.ParseFloat(matches[1], 64)
		if err == nil {
			return num * 1e9
		}
	}
	return 0
}

func main() {
	var (
		help         = flag.Bool("help", false, "Display help information")
		version      = flag.Bool("version", false, "Show tool version")
		info         = flag.Bool("info", false, "Show configuration information")
		model        = flag.String("model", "", "Specify LLM model to use")
		outputFormat = flag.String("output-format", "plain", "Set response format (plain, json, markdown)")
		configPath   = flag.String("config", "~/.llmrc", "Path to configuration file")
		systemPrompt = flag.String("system-prompt", "", "Override default system prompt")
		newSession   = flag.Bool("new-session", false, "Start a new session, flushing any existing session")
		verbose      = flag.Bool("verbose", false, "Enable verbose diagnostic output")
	)

	flag.Parse()

	// Setup signal handling for graceful shutdown on Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Check for .llmrc in location specified by user, or in current working directory, or in user homedir ~/.llmrc
	localConfig := "./.llmrc"
	var expandedConfigPath string
	if *configPath == "~/.llmrc" {
		if _, err := os.Stat(localConfig); err == nil {
			expandedConfigPath = localConfig
		} else {
			expandedConfigPath = cli.ExpandTilde(*configPath)
		}
	} else {
		expandedConfigPath = cli.ExpandTilde(*configPath)
	}

	if *help {
		flag.Usage()
		return
	}

	if *version {
		if buildTime != "" {
			fmt.Printf("LLM CLI Tool v%s (built %s)\n", VERSION, buildTime)
		} else {
			fmt.Printf("LLM CLI Tool v%s\n", VERSION)
		}
		return
	}

	if *info {
		loader := config.NewLoader()
		cfg, err := loader.Load(expandedConfigPath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, ".llmrc not found at %s. Using default configuration.\n", expandedConfigPath)
				cfg = &config.Config{}
			} else {
				fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
				os.Exit(1)
			}
		}
		if cfg.GrokAPIKey != "" {
			fmt.Println("grok_api_key=***")
		}
		if cfg.OpenAIAPIKey != "" {
			fmt.Println("openai_api_key=***")
		}
		if cfg.OpenRouterAPIKey != "" {
			fmt.Println("openrouter_api_key=***")
		}
		if cfg.LMProxyAPIKey != "" {
			fmt.Println("lm_proxy_api_key=***")
		}
		if cfg.LMProxyURL != "" {
			fmt.Printf("lm_proxy_url=%s\n", cfg.LMProxyURL)
		}
		if cfg.GrokModel != "" {
			fmt.Printf("grok_model=%s\n", cfg.GrokModel)
		}
		if cfg.OpenAIModel != "" {
			fmt.Printf("openai_model=%s\n", cfg.OpenAIModel)
		}
		if cfg.OpenRouterModel != "" {
			fmt.Printf("openrouter_model=%s\n", cfg.OpenRouterModel)
		}
		if cfg.LMProxyModel != "" {
			fmt.Printf("lm_proxy_model=%s\n", cfg.LMProxyModel)
		}
		if cfg.DefaultProvider != "" {
			fmt.Printf("default_provider=%s\n", cfg.DefaultProvider)
		}
		if cfg.SystemPrompt != "" {
			fmt.Printf("system_prompt=%s\n", cfg.SystemPrompt)
		}
		return
	}

	// Load config
	loader := config.NewLoader()
	cfg, err := loader.Load(expandedConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, ".llmrc not found at %s. Using default configuration.\n", expandedConfigPath)
			cfg = &config.Config{}
		} else {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}
	}
	if cfg.SystemPrompt == "" {
		fmt.Fprintf(os.Stderr, "No system_prompt defined in configuration. Proceeding without a system message.\n")
	}
	verboseLog(*verbose, "Loaded config from %s", expandedConfigPath)
	verboseLog(*verbose, "Config keys: grok_api_key=%s openai_api_key=%s openrouter_api_key=%s lm_proxy_api_key=%s", maskedValue(cfg.GrokAPIKey), maskedValue(cfg.OpenAIAPIKey), maskedValue(cfg.OpenRouterAPIKey), maskedValue(cfg.LMProxyAPIKey))
	verboseLog(*verbose, "Config defaults: grok_model=%s openai_model=%s openrouter_model=%s lm_proxy_model=%s default_provider=%s", cfg.GrokModel, cfg.OpenAIModel, cfg.OpenRouterModel, cfg.LMProxyModel, cfg.DefaultProvider)

	// Determine model
	selectedModel := *model
	if selectedModel == "" {
		selectedModel = cfg.DefaultProvider
		if selectedModel == "" {
			selectedModel = "openrouter"
		}
	}
	verboseLog(*verbose, "Selected provider: %s", selectedModel)

	factory := provider.NewFactory()
	prov, queryModel, err := factory.Create(selectedModel, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	verboseLog(*verbose, "Provider initialized. Query model: %s", queryModel)

	// Load or create session
	sessionManager := session.NewManager()
	sess, err := sessionManager.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading session: %v\n", err)
		os.Exit(1)
	}
	verboseLog(*verbose, "Loaded session: provider=%s messages=%d", sess.Provider, len(sess.Messages))

	// Check if new session or expired
	if *newSession || sessionManager.IsExpired(sess) || sess.Provider != selectedModel {
		sess = sessionManager.NewSession(selectedModel)
		verboseLog(*verbose, "Started new session for provider=%s", selectedModel)
	}

	// Add system prompt if first message
	if len(sess.Messages) == 0 {
		systemMsg := cfg.SystemPrompt
		if *systemPrompt != "" {
			systemMsg = *systemPrompt
		}
		if systemMsg != "" {
			systemMsg = cli.ReplacePlaceholders(systemMsg)
			sessionManager.AddMessage(sess, "system", systemMsg)
			verboseLog(*verbose, "Added system prompt")
		}
	}

	// Get query
	args := flag.Args()
	query := ""
	if len(args) > 0 {
		query = strings.Join(args, " ")
	} else {
		// Check if stdin has data (not a TTY = piped input)
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Stdin is piped - read everything
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
				os.Exit(1)
			}
			query = strings.TrimSpace(string(data))
		} else {
			// Interactive mode - read multi-line input until EOF (Ctrl+D)
			fmt.Fprintln(os.Stderr, "Enter your prompt (Ctrl+D when done):")
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
				os.Exit(1)
			}
			query = strings.TrimSpace(string(data))
		}
	}

	if query == "" {
		fmt.Fprintln(os.Stderr, "Error: No query provided")
		flag.Usage()
		os.Exit(1)
	}
	verboseLog(*verbose, "Captured user query (%d chars)", len(query))

	// Add user message
	sessionManager.AddMessage(sess, "user", query)
	if *verbose {
		promptJSON, _ := json.MarshalIndent(sessionManager.GetMessages(sess), "", "  ")
		fmt.Printf("[verbose] Prompt messages:\n%s\n", string(promptJSON))
	}

	if selectedModel == "openrouter" && cfg.OpenRouterModel == "_free_" {
		if cfg.OpenRouterAPIKey == "" {
			fmt.Fprintf(os.Stderr, "OpenRouter API key required for _free_ model.\n")
			os.Exit(1)
		}
		if sess.OpenRouterFreeModel == "" {
			ctx, cancel := context.WithTimeout(context.Background(), provider.ModelDiscoveryTimeout)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, "GET", "https://openrouter.ai/api/v1/models", nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
				os.Exit(1)
			}
			req.Header.Set("Authorization", "Bearer "+cfg.OpenRouterAPIKey)
			verboseLog(*verbose, "OpenRouter model discovery request: %s (Authorization: %s)", req.URL.String(), maskedValue(cfg.OpenRouterAPIKey))
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				if err == context.DeadlineExceeded {
					fmt.Fprintf(os.Stderr, "Error fetching models: request timed out after %v - OpenRouter may be slow or unresponsive\n", provider.ModelDiscoveryTimeout)
				} else {
					fmt.Fprintf(os.Stderr, "Error fetching models: %v\n", err)
				}
				os.Exit(1)
			}
			defer resp.Body.Close()
			verboseLog(*verbose, "OpenRouter model discovery response status: %d", resp.StatusCode)
			if resp.StatusCode != 200 {
				body, _ := io.ReadAll(resp.Body)
				fmt.Fprintf(os.Stderr, "API error: %s\n", string(body))
				os.Exit(1)
			}
			var modelsResp openrouterModelsResponse
			if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
				fmt.Fprintf(os.Stderr, "Error decoding models: %v\n", err)
				os.Exit(1)
			}
			var bestModel string
			maxParams := float64(0)
			bestContext := 0
			for _, m := range modelsResp.Data {
				if m.Pricing.Input == 0 && m.Pricing.Output == 0 {
					params := getModelParams(m.ID)
					if params > maxParams || (params == maxParams && m.ContextLength > bestContext) {
						maxParams = params
						bestContext = m.ContextLength
						bestModel = m.ID
					}
				}
			}
			if bestModel == "" {
				fmt.Fprintln(os.Stderr, "No free models (price 0 for input and output) found on OpenRouter.")
				os.Exit(1)
			}
			sess.OpenRouterFreeModel = bestModel
			if err := sessionManager.Save(sess); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving session: %v\n", err)
			}
			fmt.Printf("Selected free model: %s\n", bestModel)
		}
		queryModel = sess.OpenRouterFreeModel
		prov.(*provider.OpenRouterProvider).FreeMode = true
		verboseLog(*verbose, "OpenRouter free mode enabled with model %s", queryModel)
	}

	// Query LLM
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Suffix = " Waiting for " + selectedModel + " response..."
	s.Start()
	defer s.Stop() // Ensure spinner cleanup on all exit paths (error, panic, or normal return)

	// Handle Ctrl+C gracefully to ensure spinner cleanup
	go func() {
		<-sigChan
		s.Stop() // Clean up spinner before exit
		fmt.Fprintln(os.Stderr, "\nInterrupted by user")
		os.Exit(130) // Standard exit code for SIGINT (128 + 2)
	}()

	var resp string
	var accumulated string
	resp, err = prov.Query(queryModel, sessionManager.GetMessages(sess), func(chunk string) {
		accumulated += chunk
		wordCount := len(strings.Fields(accumulated))
		s.Suffix = fmt.Sprintf(" Waiting for %s response... (%d words)", selectedModel, wordCount)
	}, *verbose)
	s.Stop() // Stop spinner before any output
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying LLM: %v\n", err)
		os.Exit(1)
	}
	verboseLog(*verbose, "Received response (%d chars)", len(resp))
	if *verbose {
		fmt.Printf("[verbose] Response content:\n%s\n", resp)
	}

	// Add assistant response
	sessionManager.AddMessage(sess, "assistant", resp)

	err = sessionManager.Save(sess)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error saving session: %v\n", err)
	}

	// Handle response (this may execute commands and add their output to session)
	response.Handle(sessionManager, sess, query, resp, *outputFormat)
	verboseLog(*verbose, "Response handling completed")
}

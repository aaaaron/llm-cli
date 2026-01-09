/*
Package response provides response handling utilities.

Parses LLM responses to print non-code text, save code files from blocks,
and optionally execute shell commands interactively.
*/
package response

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/term"

	"llm/internal/session"
	"llm/internal/ui"
)

// languageExtensions maps programming/scripting language names to their standard file extensions.
var languageExtensions = map[string]string{
	"python":      "py",
	"javascript":  "js",
	"typescript":  "ts",
	"java":        "java",
	"c":           "c",
	"cpp":         "cpp",
	"c++":         "cpp",
	"go":          "go",
	"rust":        "rs",
	"ruby":        "rb",
	"php":         "php",
	"html":        "html",
	"css":         "css",
	"json":        "json",
	"xml":         "xml",
	"yaml":        "yaml",
	"yml":         "yml",
	"markdown":    "md",
	"sql":         "sql",
	"bash":        "sh",
	"sh":          "sh",
	"r":           "r",
	"perl":        "pl",
	"lua":         "lua",
	"scala":       "scala",
	"kotlin":      "kt",
	"swift":       "swift",
	"objective-c": "m",
	"dart":        "dart",
	"haskell":     "hs",
	"clojure":     "clj",
	"erlang":      "erl",
	"elixir":      "ex",
	"f#":          "fs",
	"vb":          "vb",
	"powershell":  "ps1",
	"shell":       "sh",
	"dockerfile":  "dockerfile",
	"makefile":    "mk",
	"tex":         "tex",
	"matlab":      "m",
	"racket":      "rkt",
	"scheme":      "scm",
	"prolog":      "pl",
	"fortran":     "f90",
	"cobol":       "cob",
	"pascal":      "pas",
	"ada":         "ada",
	"vhdl":        "vhdl",
	"verilog":     "v",
	"assembly":    "asm",
	"nasm":        "asm",
	"gas":         "s",
	"llvm":        "ll",
	"webassembly": "wat",
	"graphql":     "graphql",
	"thrift":      "thrift",
	"protobuf":    "proto",
	"toml":        "toml",
	"ini":         "ini",
	"properties":  "properties",
	"csv":         "csv",
	"tsv":         "tsv",
	"log":         "log",
	"diff":        "diff",
	"patch":       "patch",
}

// IsUserInteractive checks if stdout is a terminal (for interactive command prompts).
func IsUserInteractive() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// Handle processes the LLM response: prints formatted non-code, saves files, executes commands if interactive.
func Handle(sessionManager *session.Manager, sess *session.Session, query, response, format string) {
	width, _, err := term.GetSize(0)
	if err != nil || width < 10 {
		width = 80
	}
	visualSep := ui.GetVisualSeparator(width)
	// Print non-code response
	ui.FormatAndPrintNonCodeResponse(response, format, visualSep, width)
	// Save files from code blocks
	saveFilesFromResponse(response)

	// Offer to execute bash/sh commands if interactive
	if IsUserInteractive() {
		re := regexp.MustCompile("(?s)```([a-zA-Z0-9+#-]+)?\\r?\\n(.*?)```")
		matches := re.FindAllStringSubmatch(response, -1)
		for _, match := range matches {
			lang := strings.ToLower(strings.TrimSpace(match[1]))
			if lang != "bash" && lang != "sh" {
				continue
			}
			cmd := strings.TrimSpace(match[2])
			fmt.Printf("%s%s%s\n", ui.ColorGreen, cmd, ui.ColorReset)
			fmt.Println(visualSep)
			fmt.Printf("%sExecute this command? y/N%s ", ui.ColorCyan, ui.ColorReset)
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer == "y" || answer == "yes" {
				addToShellHistory(cmd)
				executeShellCommand(sessionManager, sess, cmd)
			}
		}
	}
}

// saveFilesFromResponse extracts code blocks from response and saves as generated_N.ext files.
// Skips bash/sh blocks (handled separately).
func saveFilesFromResponse(response string) {
	re := regexp.MustCompile("(?s)```([a-zA-Z0-9+#-]+)?\\r?\\n(.*?)```")
	matches := re.FindAllStringSubmatch(response, -1)
	savedCount := 0
	for _, match := range matches {
		lang := strings.ToLower(strings.TrimSpace(match[1]))
		code := match[2]
		if lang == "" {
			lang = "txt"
		}
		switch lang {
		case "bash", "sh":
			continue
		}
		ext := lang
		if mapped, ok := languageExtensions[lang]; ok {
			ext = mapped
		}
		savedCount++
		filename := fmt.Sprintf("generated_%d.%s", savedCount, ext)
		err := os.WriteFile(filename, []byte(code), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%sError saving file %s: %v%s\n", ui.ColorRed, filename, err, ui.ColorReset)
		} else {
			fmt.Printf("%sSaved file: %s%s\n", ui.ColorGreen, filename, ui.ColorReset)
		}
	}
}

// addCommandError adds a system message to the session with command and error details.
func addCommandError(sessionManager *session.Manager, sess *session.Session, cmd string, err error) {
	msg := fmt.Sprintf("Executed command: `%s`\nError: %v", cmd, err)
	sessionManager.AddMessage(sess, "system", "Command output: "+msg)
	if errSave := sessionManager.Save(sess); errSave != nil {
		fmt.Fprintf(os.Stderr, "Error saving session: %v\n", errSave)
	}
}

// addToShellHistory appends the command to the user's shell history file.
func addToShellHistory(cmd string) {
	histfile := os.Getenv("HISTFILE")
	if histfile == "" {
		home := os.Getenv("HOME")
		if home == "" {
			return // Cannot determine history file
		}
		shell := os.Getenv("SHELL")
		if strings.HasSuffix(shell, "/zsh") || strings.HasSuffix(shell, "zsh") {
			histfile = home + "/.zsh_history"
		} else {
			histfile = home + "/.bash_history"
		}
	}
	file, err := os.OpenFile(histfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not open history file %s: %v\n", histfile, err)
		return
	}
	defer file.Close()
	_, err = file.WriteString(cmd + "\n")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not write to history file: %v\n", err)
	}
}

// executeShellCommand runs the command via sh -c, captures stdout/stderr with colors, adds output to session.
func executeShellCommand(sessionManager *session.Manager, sess *session.Session, cmd string) {
	fmt.Println(ui.ColorBlue + "Executing..." + ui.ColorReset)
	command := exec.Command("sh", "-c", cmd)
	stdout, err := command.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError creating stdout pipe: %v%s\n", ui.ColorRed, err, ui.ColorReset)
		addCommandError(sessionManager, sess, cmd, err)
		return
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError creating stderr pipe: %v%s\n", ui.ColorRed, err, ui.ColorReset)
		addCommandError(sessionManager, sess, cmd, err)
		return
	}
	err = command.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError starting command: %v%s\n", ui.ColorRed, err, ui.ColorReset)
		addCommandError(sessionManager, sess, cmd, err)
		return
	}
	var wg sync.WaitGroup
	var outputBuilder strings.Builder
	wg.Add(2)
	go func() {
		defer wg.Done()
		reader := bufio.NewReader(stdout)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				break
			}
			line = strings.TrimRight(line, "\r\n")
			fmt.Println(ui.ColorGreen + line + ui.ColorReset)
			outputBuilder.WriteString(line + "\n")
		}
	}()
	go func() {
		defer wg.Done()
		reader := bufio.NewReader(stderr)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				break
			}
			line = strings.TrimRight(line, "\r\n")
			fmt.Fprintf(os.Stderr, "%s%s%s\n", ui.ColorRed, line, ui.ColorReset)
			outputBuilder.WriteString(line + "\n")
		}
	}()
	wg.Wait()
	err = command.Wait()
	outputStr := strings.TrimSpace(outputBuilder.String())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError: %v%s\n", ui.ColorRed, err, ui.ColorReset)
		msg := fmt.Sprintf("Executed command: `%s`\nError: %v\nOutput: %s", cmd, err, outputStr)
		sessionManager.AddMessage(sess, "system", "Command output: "+msg)
	} else {
		msg := fmt.Sprintf("Executed command: `%s`\nOutput: %s", cmd, outputStr)
		sessionManager.AddMessage(sess, "system", "Command output: "+msg)
	}
	if errSave := sessionManager.Save(sess); errSave != nil {
		fmt.Fprintf(os.Stderr, "Error saving session: %v\n", errSave)
	}
	if err == nil {
		fmt.Println()
	}
}

/*
Package ui provides UI utilities for the LLM CLI terminal interface.

Provides ANSI color constants, visual separators, and response formatting with Markdown support via glamour.
*/
package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
)

// ANSI color constants
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
)

// GetVisualSeparator returns a horizontal line separator of the given width using ─ character.
func GetVisualSeparator(width int) string {
	return strings.Repeat("─", width)
}

// FormatAndPrintNonCodeResponse formats and prints the non-code portion of the LLM response.
// Strips code blocks, supports plain, markdown (via glamour), and json formats.
// Uses terminal width for wrapping and trims excessive whitespace.
func FormatAndPrintNonCodeResponse(response, format, visualSep string, width int) {
	displayResponse := regexp.MustCompile("(?s)```.*?```").ReplaceAllString(response, "")
	displayResponse = strings.TrimSpace(displayResponse)
	var output string
	switch format {
	case "json":
		output = fmt.Sprintf(`{"response": %q}`, displayResponse)
	case "markdown", "plain":
		r, _ := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(width-4),
		)
		output, _ = r.Render(displayResponse)
		output = regexp.MustCompile(`\n{2,}`).ReplaceAllString(output, "\n\n")
		lines := strings.Split(output, "\n")
		var trimmedLines []string
		for _, line := range lines {
			trimmedLines = append(trimmedLines, strings.TrimRight(line, " \t"))
		}
		output = strings.Join(trimmedLines, "\n")
	}
	if strings.TrimSpace(displayResponse) != "" {
		fmt.Println(strings.TrimSpace(output))
		fmt.Println(visualSep)
	}
}

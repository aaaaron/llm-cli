/*
Package cli provides CLI utilities for the LLM tool.
*/
package cli

import (
	"os"
	"os/user"
	"runtime"
	"strings"
)

// ExpandTilde expands ~ to the user's home directory in a path.
func ExpandTilde(path string) string {
	if strings.HasPrefix(path, "~") {
		usr, err := user.Current()
		if err != nil {
			return path
		}
		return strings.Replace(path, "~", usr.HomeDir, 1)
	}
	return path
}

// GetOSString returns the current operating system name.
func GetOSString() string {
	return runtime.GOOS
}

// GetShell returns the name of the current shell.
func GetShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "unknown"
	}
	// Extract basename
	parts := strings.Split(shell, "/")
	return parts[len(parts)-1]
}

// ReplacePlaceholders replaces %%os_string%% and %%shell%% in the prompt.
func ReplacePlaceholders(prompt string) string {
	osStr := GetOSString()
	shell := GetShell()
	prompt = strings.ReplaceAll(prompt, "%%os_string%%", osStr)
	prompt = strings.ReplaceAll(prompt, "%%shell%%", shell)
	return prompt
}

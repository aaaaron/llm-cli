package cli

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestExpandTilde(t *testing.T) {
	tests := []struct {
		path string
	}{
		{"/absolute/path"},
		{"relative/path"},
		{"~"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := ExpandTilde(tt.path)
			if tt.path == "~" {
				if got == tt.path {
					t.Error("ExpandTilde(~) should expand home dir")
				}
			} else if got != tt.path {
				t.Errorf("ExpandTilde(%q) = %q, want unchanged", tt.path, got)
			}
		})
	}
}

func TestGetOSString(t *testing.T) {
	want := runtime.GOOS
	if got := GetOSString(); got != want {
		t.Errorf("GetOSString() = %v, want %v", got, want)
	}
}

func TestGetShell(t *testing.T) {
	os.Setenv("SHELL", "/bin/bash")
	want := "bash"
	if got := GetShell(); got != want {
		t.Errorf("GetShell() = %v, want %v", got, want)
	}
	os.Unsetenv("SHELL")
	if got := GetShell(); got != "unknown" {
		t.Errorf("GetShell() = %v, want %v", got, "unknown")
	}
}

func TestReplacePlaceholders(t *testing.T) {
	os.Setenv("SHELL", "/bin/zsh")
	prompt := "OS: %%os_string%% Shell: %%shell%%"
	got := ReplacePlaceholders(prompt)
	if !strings.Contains(got, GetOSString()) || !strings.Contains(got, "zsh") {
		t.Errorf("ReplacePlaceholders() = %v, want OS=%s Shell=zsh", got, GetOSString())
	}
	os.Unsetenv("SHELL")
}

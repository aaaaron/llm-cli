package ui

import (
	"testing"
)

func TestGetVisualSeparator(t *testing.T) {
	sep := GetVisualSeparator(80)
	if len(sep) < 80 {
		t.Errorf("GetVisualSeparator too short: %d chars", len(sep))
	}
	runes := []rune(sep)
	if len(runes) == 0 || runes[0] != '─' || runes[len(runes)-1] != '─' {
		t.Errorf("GetVisualSeparator not ─ repeated: %q", sep)
	}
}

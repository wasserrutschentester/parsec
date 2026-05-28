package ui

import (
	"testing"
)

func TestPrintDebug(t *testing.T) {
	// This is hard to test because it prints to stdout/stderr using lipgloss.
	// But we can at least check if it doesn't crash.
	IsDebug = true
	PrintDebug("test debug message")
	IsDebug = false
	PrintDebug("test debug message hidden")
}

func TestFormatDebug(t *testing.T) {
	msg := "test message"
	formatted := FormatDebug(msg)
	if formatted == "" {
		t.Error("FormatDebug returned empty string")
	}
}

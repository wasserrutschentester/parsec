package ui

import (
	"charm.land/lipgloss/v2"
)

var (
	// Base Colors
	blue   = lipgloss.Color("12")
	green  = lipgloss.Color("10")
	yellow = lipgloss.Color("11")
	red    = lipgloss.Color("9")
	gray   = lipgloss.Color("8")
	purple = lipgloss.Color("63")

	// Functional Styles
	Info    = lipgloss.NewStyle().Foreground(blue)
	Success = lipgloss.NewStyle().Foreground(green)
	Warning = lipgloss.NewStyle().Foreground(yellow)
	Error   = lipgloss.NewStyle().Foreground(red)
	Muted   = lipgloss.NewStyle().Foreground(gray)

	// Structural Styles
	Header = lipgloss.NewStyle().
		Bold(true).
		Foreground(purple).
		MarginBottom(1).
		Underline(true)

	Label = lipgloss.NewStyle().
		Bold(true).
		Foreground(blue).
		Width(20)

	Value = lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")) // White

	// Components
	WarningTag = lipgloss.NewStyle().
			Bold(true).
			Background(yellow).
			Foreground(lipgloss.Color("0")).
			Padding(0, 1).
			MarginRight(1).
			Render("WARNING")

	ErrorTag = lipgloss.NewStyle().
			Bold(true).
			Background(red).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1).
			MarginRight(1).
			Render("ERROR")
)

// Helper functions for common patterns
func LabelValue(label, value string) string {
	return Label.Render(label) + Value.Render(value)
}

func FormatWarning(msg string) string {
	return WarningTag + Warning.Render(msg)
}

func FormatError(msg string) string {
	return ErrorTag + Error.Render(msg)
}

func PrintWarning(msg string) {
	lipgloss.Println(FormatWarning(msg))
}

func PrintError(msg string) {
	lipgloss.Println(FormatError(msg))
}

func Println(a ...any) {
	lipgloss.Println(a...)
}

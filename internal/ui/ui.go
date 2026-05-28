package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"codeberg.org/n0ne/parsec/internal/types"
)

var (
	// IsSilent suppresses all UI output
	IsSilent bool

	// Base Colors
	blue   = lipgloss.Color("12")
	green  = lipgloss.Color("10")
	yellow = lipgloss.Color("11")
	red    = lipgloss.Color("9")
	gray   = lipgloss.Color("8")
	purple = lipgloss.Color("63")
	white  = lipgloss.Color("15")

	// Functional Styles
	Info    = lipgloss.NewStyle().Foreground(blue)
	Success = lipgloss.NewStyle().Foreground(green)
	Warning = lipgloss.NewStyle().Foreground(yellow)
	Error   = lipgloss.NewStyle().Foreground(red)
	Muted   = lipgloss.NewStyle().Foreground(gray)
	Link    = lipgloss.NewStyle().Foreground(blue).Underline(true)

	// Structural Styles
	Header = lipgloss.NewStyle().
		Bold(true).
		Foreground(purple).
		MarginBottom(1).
		Underline(true)

	LabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(blue)

	ValueStyle = lipgloss.NewStyle().
			Foreground(white)

	// Icons
	IconCheck = Success.Render("✓")
	IconCross = Error.Render("✗")
	IconWarn  = Warning.Render("!")
	IconInfo  = Info.Render("i")
	IconArrow = Info.Render("→")

	// Component Styles
	BadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1).
			MarginRight(1)

	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(purple).
			Padding(0, 1)

	// Legacy Styles (keeping for compatibility)
	Label = lipgloss.NewStyle().
		Bold(true).
		Foreground(blue).
		Width(20)

	Value = lipgloss.NewStyle().
		Foreground(white)

	WarningTag = BadgeStyle.
			Background(yellow).
			Foreground(lipgloss.Color("0")).
			Render("WARNING")

	ErrorTag = BadgeStyle.
			Background(red).
			Foreground(white).
			Render("ERROR")
)

// Status Badges
func SuccessBadge(msg string) string {
	return BadgeStyle.Background(green).Foreground(lipgloss.Color("0")).Render("OK") + " " + Success.Render(msg)
}

func WarningBadge(msg string) string {
	return WarningTag + " " + Warning.Render(msg)
}

func ErrorBadge(msg string) string {
	return ErrorTag + " " + Error.Render(msg)
}

// Card renders a visual card with title, subtitle, body and footer.
func Card(title, subtitle, body, footer string) string {
	width := 80 // Default width

	t := lipgloss.NewStyle().Bold(true).Foreground(purple).Render(title)
	s := lipgloss.NewStyle().Foreground(yellow).Render(subtitle)

	header := lipgloss.JoinHorizontal(lipgloss.Top, t, strings.Repeat(" ", max(0, width-lipgloss.Width(t)-lipgloss.Width(s))), s)
	divider := lipgloss.NewStyle().Foreground(gray).Render(strings.Repeat("─", width))

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		divider,
		"",
		body,
		"",
		Muted.Render(footer),
	)

	return CardStyle.Width(width + 2).Render(content)
}

// FormatDiff visualizes a mismatch between an expected and actual value.
func FormatDiff(expectedLabel, expectedValue, actualLabel, actualValue string) string {
	expectedLine := lipgloss.NewStyle().Foreground(green).Render(fmt.Sprintf("+ %s: %s", expectedLabel, expectedValue))
	actualLine := lipgloss.NewStyle().Foreground(red).Render(fmt.Sprintf("- %s: %s", actualLabel, actualValue))
	return lipgloss.JoinVertical(lipgloss.Left, actualLine, expectedLine)
}

// TrackTable renders a table of track information.
func TrackTable(headers []string, rows [][]string) string {
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(gray)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row < 0 { // Header row
				return lipgloss.NewStyle().Bold(true).Foreground(blue).Align(lipgloss.Center)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Headers(headers...).
		Rows(rows...)

	return t.Render()
}

// FormatTrackTable renders a table of track issues.
func FormatTrackTable(tracks []types.TrackCheckResult) string {
	if len(tracks) == 0 {
		return ""
	}

	headers := []string{"ID", "Type", "#", "Codec", "Lang", "Name", "Flags", "Warning"}
	var rows [][]string
	for _, t := range tracks {
		rows = append(rows, []string{
			t.ID,
			t.Type,
			fmt.Sprintf("%d", t.TypeOrder),
			t.Codec,
			t.Language,
			t.Name,
			strings.Join(t.Flags, ", "),
			Warning.Render(t.Warning),
		})
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(gray)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row < 0 { // Header row
				return lipgloss.NewStyle().Bold(true).Foreground(blue).Align(lipgloss.Center)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Headers(headers...).
		Rows(rows...)

	return "      " + strings.ReplaceAll(t.Render(), "\n", "\n      ")
}

// ReportSection returns a header for a specific section in a check report.
func ReportSection(name string) string {
	return "\n" + lipgloss.NewStyle().
		Bold(true).
		Foreground(blue).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		Render(fmt.Sprintf("[ %s ]", strings.ToUpper(name)))
}

// PropertyLayout takes pairs of labels and values and aligns them.
func PropertyLayout(pairs [][2]string) string {
	if len(pairs) == 0 {
		return ""
	}

	maxLabelLen := 0
	for _, p := range pairs {
		if len(p[0]) > maxLabelLen {
			maxLabelLen = len(p[0])
		}
	}

	var lines []string
	for _, p := range pairs {
		label := LabelStyle.Width(maxLabelLen + 2).Render(p[0] + ":")
		value := ValueStyle.Render(p[1])
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, label, value))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// Helper functions for common patterns
func LabelValue(label, value string) string {
	return Label.Render(label) + Value.Render(value)
}

func FormatWarning(msg string) string {
	return WarningTag + " " + Warning.Render(msg)
}

func FormatError(msg string) string {
	return ErrorTag + " " + Error.Render(msg)
}

func PrintWarning(msg string) {
	if !IsSilent {
		lipgloss.Println(FormatWarning(msg))
	}
}

func PrintError(msg string) {
	fmt.Fprintln(os.Stderr, FormatError(msg))
}

func Println(a ...any) {
	if !IsSilent {
		lipgloss.Println(a...)
	}
}

// ConfirmContinue prompts the user to continue.
// Empty input returns true, "no" returns false.
func ConfirmContinue(msg string) bool {
	if IsSilent || !IsTerminal() {
		return true
	}

	fmt.Printf("%s [Y/n]: ", msg)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		input := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if input == "no" || input == "n" {
			return false
		}
	}
	return true
}

// IsTerminal returns true if both stdin and stdout are terminals.
func IsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		return false
	}

	fi, err = os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

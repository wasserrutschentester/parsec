package ui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/aymanbagabas/go-udiff"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"

	"codeberg.org/upPollo/parsec/internal/types"
)

var (
	// IsSilent suppresses all UI output
	IsSilent bool

	// IsDebug enables debug output
	IsDebug bool

	// Base Colors
	blue   = lipgloss.Color("51")
	green  = lipgloss.Color("118")
	yellow = lipgloss.Color("11")
	red    = lipgloss.Color("196")
	gray   = lipgloss.Color("244")
	purple = lipgloss.Color("141")
	white  = lipgloss.Color("255")
	black  = lipgloss.Color("0")

	// Functional Styles

	// Info is the style for information messages.
	Info = lipgloss.NewStyle().Foreground(blue)
	// Success is the style for success messages.
	Success = lipgloss.NewStyle().Foreground(green)
	// Warning is the style for warning messages.
	Warning = lipgloss.NewStyle().Foreground(yellow)
	// Error is the style for error messages.
	Error = lipgloss.NewStyle().Foreground(red)
	// Muted is the style for muted/secondary text.
	Muted = lipgloss.NewStyle().Foreground(gray)
	// Debug is the style for debug messages.
	Debug = lipgloss.NewStyle().Foreground(gray)
	// Link is the style for clickable-like links.
	Link = lipgloss.NewStyle().Foreground(blue).Underline(true)

	// Structural Styles

	// Header is the style for section headers.
	Header = lipgloss.NewStyle().
		Bold(true).
		Foreground(purple).
		MarginBottom(1).
		BorderStyle(lipgloss.ThickBorder()).
		BorderBottom(true).
		BorderForeground(purple)

	// LabelStyle is the style for property labels.
	LabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(blue)

	// ValueStyle is the style for property values.
	ValueStyle = lipgloss.NewStyle().
			Foreground(white)

	// Icons

	// IconCheck is a success checkmark icon.
	IconCheck = Success.Render("✓")
	// IconCross is a failure cross icon.
	IconCross = Error.Render("✗")

	// Component Styles

	// BadgeStyle is the base style for badges.
	BadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1).
			MarginRight(1)

	// CardStyle is the base style for cards.
	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(purple).
			Padding(0, 1)

	// Legacy Styles (keeping for compatibility)

	// Label is the legacy style for labels.
	Label = lipgloss.NewStyle().
		Bold(true).
		Foreground(blue).
		Width(20)

	// Value is the legacy style for values.
	Value = lipgloss.NewStyle().
		Foreground(white)

	// WarningTag is a pre-rendered warning badge.
	WarningTag = BadgeStyle.
			Background(yellow).
			Foreground(black).
			Render("ANOMALY")

	// ErrorTag is a pre-rendered error badge.
	ErrorTag = BadgeStyle.
			Background(red).
			Foreground(white).
			Render("CRITICAL")

	// DebugTag is a pre-rendered debug badge.
	DebugTag = BadgeStyle.
			Background(gray).
			Foreground(white).
			Render("DEBUG")
)

// DisableColors globally disables color output for all styles and components.
func DisableColors() {
	// In v2, global color downsampling is handled by the Writer.
	// Note: Style.Render() still produces ANSI codes, so we must also reset styles.
	lipgloss.Writer.Profile = colorprofile.ASCII

	// Reset base colors
	blue = lipgloss.NoColor{}
	green = lipgloss.NoColor{}
	yellow = lipgloss.NoColor{}
	red = lipgloss.NoColor{}
	gray = lipgloss.NoColor{}
	purple = lipgloss.NoColor{}
	white = lipgloss.NoColor{}
	black = lipgloss.NoColor{}

	// Reset functional styles
	Info = lipgloss.NewStyle()
	Success = lipgloss.NewStyle()
	Warning = lipgloss.NewStyle()
	Error = lipgloss.NewStyle()
	Muted = lipgloss.NewStyle()
	Debug = lipgloss.NewStyle()
	Link = lipgloss.NewStyle()

	// Reset structural styles
	Header = lipgloss.NewStyle().Bold(true)
	LabelStyle = lipgloss.NewStyle().Bold(true)
	ValueStyle = lipgloss.NewStyle()
	Label = lipgloss.NewStyle().Bold(true)
	Value = lipgloss.NewStyle()
	BadgeStyle = lipgloss.NewStyle().Bold(true)
	CardStyle = lipgloss.NewStyle()

	// Re-render global icons/tags
	IconCheck = Success.Render("✓")
	IconCross = Error.Render("✗")
	WarningTag = BadgeStyle.Render("ANOMALY")
	ErrorTag = BadgeStyle.Render("CRITICAL")
	DebugTag = BadgeStyle.Render("DEBUG")
}

// Card renders a visual card with title, subtitle, body and footer.
func Card(title, subtitle, body, footer string) string {
	// Determine card width based on terminal size
	width, _, _ := term.GetSize(os.Stdout.Fd())
	if width <= 0 {
		width = 80
	}

	if width > 100 {
		width = 100
	}

	// Overhead: 2 for borders, 2 for padding
	const overhead = 4

	innerWidth := width - overhead
	if innerWidth < 40 {
		innerWidth = 40
		width = innerWidth + overhead
	}

	t := lipgloss.NewStyle().Bold(true).Foreground(purple).Render(title)
	s := lipgloss.NewStyle().Foreground(yellow).Render(subtitle)

	// Header alignment
	spaceWidth := max(0, innerWidth-lipgloss.Width(t)-lipgloss.Width(s))
	header := lipgloss.JoinHorizontal(lipgloss.Top, t, strings.Repeat(" ", spaceWidth), s)
	divider := lipgloss.NewStyle().Foreground(gray).Render(strings.Repeat("─", innerWidth))

	// Ensure body and footer are wrapped to innerWidth
	bodyStyle := lipgloss.NewStyle().Width(innerWidth)
	footerStyle := Muted.Width(innerWidth)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		divider,
		"",
		bodyStyle.Render(body),
		"",
		footerStyle.Render(footer),
	)

	return CardStyle.Width(width).Render(content)
}

// formatStringDiff visualizes a mismatch between two strings with character-level alignment.
func formatStringDiff(oldStr, newStr string) string {
	edits := udiff.Strings(oldStr, newStr)

	var line1, line2 strings.Builder

	pos := 0

	for _, edit := range edits {
		// Add unchanged part
		if edit.Start > pos {
			unchanged := oldStr[pos:edit.Start]
			line1.WriteString(unchanged)
			line2.WriteString(unchanged)
		}

		oldText := oldStr[edit.Start:edit.End]
		newText := edit.New

		maxW := calculateMaxW(oldText, newText)

		renderOldPart(&line1, oldText, maxW)
		renderNewPart(&line2, newText, maxW)

		pos = edit.End
	}

	// Add remaining unchanged part
	if pos < len(oldStr) {
		remaining := oldStr[pos:]
		line1.WriteString(remaining)
		line2.WriteString(remaining)
	}

	return lipgloss.JoinVertical(lipgloss.Left, line1.String(), line2.String())
}

func calculateMaxW(oldText, newText string) int {
	oldW := lipgloss.Width(oldText)

	newW := lipgloss.Width(newText)
	if newW > oldW {
		return newW
	}

	return oldW
}

func renderOldPart(b *strings.Builder, oldText string, maxW int) {
	oldW := lipgloss.Width(oldText)
	if oldW > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(red).Render(oldText))

		if maxW > oldW {
			b.WriteString(strings.Repeat(" ", maxW-oldW))
		}
	} else if maxW > 0 {
		b.WriteString(strings.Repeat(" ", maxW))
	}
}

func renderNewPart(b *strings.Builder, newText string, maxW int) {
	newW := lipgloss.Width(newText)
	if newW > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(green).Render(newText))

		if maxW > newW {
			b.WriteString(strings.Repeat(" ", maxW-newW))
		}
	} else if maxW > 0 {
		b.WriteString(strings.Repeat(" ", maxW))
	}
}

// FormatStringDiffAligned visualizes a mismatch between two strings with labels and character-level alignment.
func FormatStringDiffAligned(expectedLabel, expectedValue, actualLabel, actualValue string) string {
	diff := formatStringDiff(expectedValue, actualValue)

	lines := strings.Split(diff, "\n")
	if len(lines) != 2 {
		return diff
	}

	maxLabelLen := max(len(expectedLabel), len(actualLabel))
	expectedPrefix := LabelStyle.Width(maxLabelLen + 4).Render(expectedLabel + ":")
	actualPrefix := LabelStyle.Width(maxLabelLen + 4).Render(actualLabel + ":")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top, expectedPrefix, lines[0]),
		lipgloss.JoinHorizontal(lipgloss.Top, actualPrefix, lines[1]),
	)
}

// TrackTable renders a table of track information.
func TrackTable(headers []string, rows [][]string) string {
	t := table.New().
		Border(lipgloss.DoubleBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(white)).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row < 0 { // Header row
				return lipgloss.NewStyle().Bold(true).Foreground(blue).Align(lipgloss.Center)
			}

			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Headers(headers...).
		Rows(rows...)

	return t.Render()
}

// FontComplianceTable renders a table of font compliance warning details.
func FontComplianceTable(headers []string, rows [][]string) string {
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(white)).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row < 0 { // Header row
				return lipgloss.NewStyle().Bold(true).Foreground(blue).Align(lipgloss.Center)
			}

			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Headers(headers...).
		Rows(rows...)

	return t.Render()
}

// calculateTrackTableWidths calculates the maximum width for each column across all provided tracks.
// It returns a map of column index to its content width (excluding padding and borders).
func calculateTrackTableWidths(tracks []types.TrackCheckResult) map[int]int {
	headers := []string{"ID", "Type", "#", "Codec", "Lang", "Name", "Flags", "Warning"}
	widths := make(map[int]int)

	for i, h := range headers {
		widths[i] = lipgloss.Width(h)
	}

	for _, t := range tracks {
		row := []string{
			t.ID,
			t.Type,
			strconv.Itoa(t.TypeOrder),
			t.Codec,
			t.Language,
			t.Name,
			strings.Join(t.Flags, ", "),
			t.Warning,
		}
		for i, cell := range row {
			w := lipgloss.Width(cell)
			if w > widths[i] {
				widths[i] = w
			}
		}
	}

	return widths
}

// formatTrackTable renders a table of track issues.
func formatTrackTable(tracks []types.TrackCheckResult, sharedWidths map[int]int) string {
	if len(tracks) == 0 {
		return ""
	}

	headers := []string{"ID", "Type", "#", "Codec", "Lang", "Name", "Flags", "Warning"}
	rows := getTrackRows(tracks)

	// Use shared widths if provided, otherwise calculate for this set of tracks
	contentWidths := sharedWidths
	if contentWidths == nil {
		contentWidths = calculateTrackTableWidths(tracks)
	}

	flexWidths := calculateFlexibleColumnWidths(headers, contentWidths)

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(white)).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := lipgloss.NewStyle().Padding(0, 1)
			if row < 0 { // Header row
				style = style.Bold(true).Foreground(blue).Align(lipgloss.Center)
			}

			w := flexWidths[col]

			// Fixed width including padding (+2) to ensure alignment
			return style.Width(w + 2)
		}).
		Headers(headers...).
		Rows(rows...).
		Wrap(true)

	return "      " + strings.ReplaceAll(t.Render(), "\n", "\n      ")
}

func getTrackRows(tracks []types.TrackCheckResult) [][]string {
	rows := make([][]string, 0, len(tracks))
	for _, t := range tracks {
		rows = append(rows, []string{
			t.ID,
			t.Type,
			strconv.Itoa(t.TypeOrder),
			t.Codec,
			t.Language,
			t.Name,
			strings.Join(t.Flags, ", "),
			t.Warning,
		})
	}

	return rows
}

func calculateFlexibleColumnWidths(headers []string, contentWidths map[int]int) map[int]int {
	// Determine available width
	termWidth, _, _ := term.GetSize(os.Stdout.Fd())
	if termWidth <= 0 {
		termWidth = 120 // Default fallback
	}

	// Overhead: 6 spaces indentation + 1 border per column + 1 final border + 2 padding per column
	overhead := 6 + len(headers) + 1 + (len(headers) * 2)

	availableWidth := termWidth - overhead

	flexWidths := make(map[int]int)
	fixedColsWidth := 0
	flexIndices := map[int]bool{5: true, 7: true} // Name (5) and Warning (7) are flexible

	for i := range headers {
		if !flexIndices[i] {
			flexWidths[i] = contentWidths[i]
			fixedColsWidth += contentWidths[i]
		}
	}

	remainingWidth := max(availableWidth-fixedColsWidth, 40) // Guarantee at least 40 chars for flex cols

	// Calculate total requested content width for flex columns
	totalFlexContentWidth := contentWidths[5] + contentWidths[7]

	if totalFlexContentWidth <= remainingWidth {
		// Both fit within their content width
		flexWidths[5] = contentWidths[5]
		flexWidths[7] = contentWidths[7]
	} else {
		// Proportionally distribute remaining width, but guarantee minimums
		minName := 15
		minWarning := 20

		if remainingWidth < minName+minWarning {
			flexWidths[5] = minName
			flexWidths[7] = minWarning
		} else {
			// Weighted distribution based on content length
			ratio := float64(contentWidths[5]) / float64(totalFlexContentWidth)
			flexWidths[5] = max(int(float64(remainingWidth)*ratio), minName)
			flexWidths[7] = max(remainingWidth-flexWidths[5], minWarning)
		}
	}

	return flexWidths
}

// ReportSection returns a header for a specific section in a check report.
func ReportSection(name string) string {
	return "\n" + lipgloss.NewStyle().
		Bold(true).
		Foreground(blue).
		Border(lipgloss.DoubleBorder()).
		BorderForeground(blue).
		Render(fmt.Sprintf(" %s ", strings.ToUpper(name)))
}

// Banner returns a themed ASCII art header with a custom tagline.
func Banner(tagline string) string {
	banner := `    ____
   / __ \____ __________ ___  _____
  / /_/ / __ ` + "`" + `/ ___/ ___/ _ \/ ___/
 / ____/ /_/ / /  (__  )  __/ /__
/_/    \__,_|_|  |___/\___|\___/`

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(purple).
		Render(banner)

	sub := lipgloss.NewStyle().
		Foreground(blue).
		Bold(true).
		Render("     " + tagline)

	return lipgloss.JoinVertical(lipgloss.Center, title, sub, "")
}

// PropertyLayout takes pairs of labels and values and aligns them.
func PropertyLayout(pairs [][2]string) string {
	if len(pairs) == 0 {
		return ""
	}

	// Determine available width
	width, _, _ := term.GetSize(os.Stdout.Fd())
	if width <= 0 {
		width = 80
	}

	if width > 100 {
		width = 100
	}

	innerWidth := width - 4 // Match Card inner width

	maxLabelLen := 0
	for _, p := range pairs {
		if len(p[0]) > maxLabelLen {
			maxLabelLen = len(p[0])
		}
	}

	labelWidth := maxLabelLen + 2

	valueWidth := max(innerWidth-labelWidth, 20)

	var lines []string

	for _, p := range pairs {
		label := LabelStyle.Width(labelWidth).Render(p[0] + ":")
		value := ValueStyle.Width(valueWidth).Render(p[1])
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, label, value))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// LabelValue returns a formatted string with a bold label and a white value.
func LabelValue(label, value string) string {
	return Label.Render(label) + Value.Render(value)
}

// FormatWarning returns a formatted warning message with a badge.
func FormatWarning(msg string) string {
	return WarningTag + " " + Warning.Render(msg)
}

func formatError(msg string) string {
	return ErrorTag + " " + Error.Render(msg)
}

func formatDebug(msg string) string {
	return DebugTag + " " + Debug.Render(msg)
}

// PrintWarning prints a warning message to stdout.
func PrintWarning(msg string) {
	if !IsSilent {
		_, _ = lipgloss.Println(FormatWarning(msg))
	}
}

// PrintError prints an error message to stderr.
func PrintError(msg string) {
	_, _ = lipgloss.Fprintln(os.Stderr, formatError(msg))
}

// PrintDebug prints a debug message to stdout if debug output is enabled.
func PrintDebug(msg string) {
	if IsDebug && !IsSilent {
		_, _ = lipgloss.Println(formatDebug(msg))
	}
}

// PrintSuccess prints a success message to stdout.
func PrintSuccess(msg string) {
	if !IsSilent {
		_, _ = lipgloss.Println(Success.Render("✓ ") + msg)
	}
}

// PrintInfo prints an info message to stdout.
func PrintInfo(msg string) {
	if !IsSilent {
		_, _ = lipgloss.Println(Info.Render("i ") + msg)
	}
}

// AnonymizePath replaces the user's home directory with a tilde (~).
func AnonymizePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	if after, ok := strings.CutPrefix(path, home); ok {
		return "~" + after
	}

	return path
}

// Println prints the given arguments to stdout if output is not suppressed.
func Println(a ...any) {
	if !IsSilent {
		_, _ = lipgloss.Println(a...)
	}
}

// ConfirmContinue prompts the user to continue with a [Y/n] prompt.
// Empty input returns true, "no" or "n" returns false.
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

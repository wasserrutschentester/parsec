package nfo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"codeberg.org/upPollo/parsec/internal/ui"
)

var (
	parseErrorRegex   = regexp.MustCompile(`template: (.*?):(\d+): (.*)`)
	executeErrorRegex = regexp.MustCompile(`template: (.*?):(\d+):(\d+): executing "(.*?)" at <(.*?)>: (.*)`)
	boldStyle         = lipgloss.NewStyle().Bold(true)

	// ErrTemplateSyntax is returned when a template fails to parse.
	ErrTemplateSyntax = errors.New("template syntax error")
	// ErrTemplateExecution is returned when a template fails to execute.
	ErrTemplateExecution = errors.New("template execution error")
)

// FormatTemplateError intercepts standard text/template errors and formats them nicely.
func FormatTemplateError(err error, configDirs []string) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// 1. Check for Runtime Execution Errors
	if matches := executeErrorRegex.FindStringSubmatch(errStr); matches != nil {
		return formatExecutionError(matches, configDirs)
	}

	// 2. Check for Syntax/Parse Errors
	if matches := parseErrorRegex.FindStringSubmatch(errStr); matches != nil {
		return formatParseError(matches, configDirs)
	}

	// If it doesn't match our regexes, return it as-is
	return err
}

func formatExecutionError(matches []string, configDirs []string) error {
	tmplName := matches[1]
	lineStr := matches[2]
	token := matches[5]
	msg := matches[6]

	lineNum, _ := strconv.Atoi(lineStr)

	var sb strings.Builder

	fmt.Fprintf(&sb, "\n%s\n", ui.FormatWarning(fmt.Sprintf("Template Execution Error in %s (Line %d)", boldStyle.Render(tmplName), lineNum)))
	fmt.Fprintf(&sb, "Failed at token %s: %s\n", ui.Info.Render("<"+token+">"), msg)

	snippet := getTemplateSnippet(tmplName, lineNum, configDirs)
	if snippet != "" {
		sb.WriteString("\n" + snippet)
	}

	return fmt.Errorf("%w\n%s", ErrTemplateExecution, sb.String())
}

func formatParseError(matches []string, configDirs []string) error {
	tmplName := matches[1]
	lineStr := matches[2]
	msg := matches[3]

	lineNum, _ := strconv.Atoi(lineStr)

	var sb strings.Builder

	fmt.Fprintf(&sb, "\n%s\n", ui.FormatWarning(fmt.Sprintf("Template Syntax Error in %s (Line %d)", boldStyle.Render(tmplName), lineNum)))

	if strings.Contains(msg, "unexpected EOF") {
		sb.WriteString("You likely opened a block (like `if` or `range`) but forgot to close it with `{{ end }}`.\n")
	} else {
		sb.WriteString(msg + "\n")
	}

	snippet := getTemplateSnippet(tmplName, lineNum, configDirs)
	if snippet != "" {
		sb.WriteString("\n" + snippet)
	}

	return fmt.Errorf("%w\n%s", ErrTemplateSyntax, sb.String())
}

func getTemplateSnippet(tmplName string, lineNum int, configDirs []string) string {
	content := readTemplateContent(tmplName, configDirs)
	if len(content) == 0 {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	if lineNum > len(lines) || lineNum < 1 {
		return ""
	}

	var snippet strings.Builder

	start := max(0, lineNum-2)
	end := min(len(lines), lineNum+1)

	for i := start; i < end; i++ {
		prefix := "   | "
		if i+1 == lineNum {
			prefix = ui.Error.Render("-> | ")
		}

		fmt.Fprintf(&snippet, "%s%s\n", prefix, lines[i])
	}

	return snippet.String()
}

func readTemplateContent(tmplName string, configDirs []string) []byte {
	// 1. Check config dirs first (user overrides)
	for _, dir := range configDirs {
		for _, sub := range []string{"", "partial"} {
			path := filepath.Join(dir, "nfo", sub, tmplName)
			if !strings.HasSuffix(path, ".tmpl") {
				path += ".tmpl"
			}

			if content, err := os.ReadFile(path); err == nil {
				return content
			}
		}
	}

	// 2. Check built-ins if not found
	for _, sub := range []string{"templates", "templates/partial"} {
		path := filepath.Join(sub, tmplName)
		if !strings.HasSuffix(path, ".tmpl") {
			path += ".tmpl"
		}

		if content, err := builtinTemplates.ReadFile(path); err == nil {
			return content
		}
	}

	return nil
}

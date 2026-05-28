package checks

import (
	"fmt"
	"regexp"
	"strings"

	"codeberg.org/n0ne/parsec/internal/config"
	"codeberg.org/n0ne/parsec/internal/metadata"
)

func RunFilenameChecks(name string, meta *metadata.Metadata) []CheckResult {
	var results []CheckResult

	if config.IsCheckEnabled("filename_generation_mismatch") {
		if name != meta.String() {
			results = append(results, CheckResult{
				Identifier: "filename_generation_mismatch",
				Warning:    "Generated name does not match the original",
				Passed:     false,
				Severity:   "warning",
				Expected:   name,
				Actual:     meta.String(),
			})
		}
	}

	if config.IsCheckEnabled("filename_characters") {
		if match, cleanName := findNotAllowedCharacters(name); match != "" {
			results = append(results, CheckResult{
				Identifier: "filename_characters",
				Passed:     false,
				Severity:   "warning",
				Warning:    fmt.Sprintf("disallowed character found: %s", match),
				Expected:   cleanName,
				Actual:     name,
			})
		}
	}

	if config.IsCheckEnabled("filename_sequences") {
		if match, cleanName := findCharacterSequences(name); match != "" {
			results = append(results, CheckResult{
				Identifier: "filename_sequences",
				Passed:     false,
				Severity:   "warning",
				Warning:    fmt.Sprintf("disallowed character sequence found: %s", match),
				Expected:   cleanName,
				Actual:     name,
			})
		}
	}

	return results
}

func findNotAllowedCharacters(filename string) (string, string) {
	re := regexp.MustCompile(`[^a-zA-Z0-9\-\.]`)
	match := re.FindStringSubmatch(filename)
	cleanFilename := re.ReplaceAllString(filename, "_")
	if match != nil {
		return strings.Join(match, " "), cleanFilename
	}
	return "", filename
}

func findCharacterSequences(filename string) (string, string) {
	re := regexp.MustCompile(`\.-*\.+`)
	match := re.FindStringSubmatch(filename)
	cleanFilename := re.ReplaceAllString(filename, "_")
	if match != nil {
		return strings.Join(match, " "), cleanFilename
	}
	return "", filename
}

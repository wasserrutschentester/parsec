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
		if msg := CheckAllowedCharacters(name); msg != "" {
			results = append(results, CheckResult{
				Identifier:  "filename_characters",
				Description: "Check for allowed characters in filename",
				Passed:      false,
				Severity:    "warning",
				Warning:     msg,
			})
		}
	}

	if config.IsCheckEnabled("filename_sequences") {
		if msg := CheckCharacterSequences(name); msg != "" {
			results = append(results, CheckResult{
				Identifier:  "filename_sequences",
				Description: "Check for invalid character sequences",
				Passed:      false,
				Severity:    "warning",
				Warning:     msg,
			})
		}
	}

	return results
}

func CheckAllowedCharacters(filename string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9\-\.]`)
	match := re.FindStringSubmatch(filename)
	if match != nil {
		return fmt.Sprintf("disallowed character found: %s", strings.Join(match, " "))
	}
	return ""
}

func CheckCharacterSequences(filename string) string {
	re := regexp.MustCompile(`\.-*\.+`)
	match := re.FindStringSubmatch(filename)
	if match != nil {
		return fmt.Sprintf("disallowed character sequence found: %s", strings.Join(match, " "))
	}
	return ""
}

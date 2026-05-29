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
		if name != meta.GetReleaseName() {
			results = append(results, CheckResult{
				Identifier: "filename_generation_mismatch",
				Warning:    "Generated name does not match the original",
				Passed:     false,
				Severity:   "warning",
				Expected:   name,
				Actual:     meta.GetReleaseName(),
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

	if config.IsCheckEnabled("filename_year_missing") {
		if meta.Year == 0 && !meta.IsTV {
			results = append(results, CheckResult{
				Identifier: "filename_year_missing",
				Passed:     false,
				Severity:   "warning",
				Warning:    "year is missing for this Movie",
			})
		}
	}

	if config.IsCheckEnabled("filename_year_redundant") {
		if meta.Year > 0 && meta.Season > 1900 {
			results = append(results, CheckResult{
				Identifier: "filename_year_redundant",
				Passed:     false,
				Severity:   "info",
				Warning:    fmt.Sprintf("redundant Year: The Season (%d) already indicates the year", meta.Season),
			})
		}
	}

	if config.IsCheckEnabled("filename_streaming") {
		isWeb := strings.Contains(meta.Source, "WEB")
		if isWeb && meta.Service == "" {
			results = append(results, CheckResult{
				Identifier: "filename_streaming",
				Passed:     false,
				Severity:   "warning",
				Warning:    "Streaming Service Tag is missing for WEB source",
			})
		} else if !isWeb && meta.Service != "" {
			results = append(results, CheckResult{
				Identifier: "filename_streaming",
				Passed:     false,
				Severity:   "info",
				Warning:    "Streaming Service Tag is not supported for non-WEB source",
			})
		}
	}

	if config.IsCheckEnabled("filename_tv_special") {
		if meta.IsTV && meta.Season == 0 {
			if meta.Date == "" || meta.EpisodeTitle == "" {
				warning := ""
				if meta.Date == "" {
					warning = "Date"
				} else if meta.EpisodeTitle != "" {
					warning = "Episode Title"
				} else {
					warning = "Date and Episode Title"
				}

				results = append(results, CheckResult{
					Identifier: "filename_tv_special",
					Passed:     false,
					Severity:   "warning",
					Warning:    fmt.Sprintf("%s is missing for TV Special", warning),
				})
			}
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

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
		results = append(results, checkNameMismatch(name, meta)...)
	}

	if config.IsCheckEnabled("filename_characters") {
		results = append(results, checkAllowedCharacters(name)...)
	}

	if config.IsCheckEnabled("filename_sequences") {
		results = append(results, checkCharacterSequences(name)...)
	}

	if config.IsCheckEnabled("filename_year_missing") {
		results = append(results, checkYearMissing(meta)...)
	}

	if config.IsCheckEnabled("filename_year_redundant") {
		results = append(results, checkYearRedundant(meta)...)
	}

	if config.IsCheckEnabled("filename_streaming") {
		results = append(results, checkStreamingService(meta)...)
	}

	if config.IsCheckEnabled("filename_tv_special") {
		results = append(results, checkTvSpecial(meta)...)
	}

	return results
}

func checkNameMismatch(name string, meta *metadata.Metadata) []CheckResult {
	if name != meta.GetReleaseName() {
		return []CheckResult{{
			Identifier: "filename_generation_mismatch",
			Warning:    "Generated name does not match the original",
			Passed:     false,
			Severity:   "warning",
			Expected:   name,
			Actual:     meta.GetReleaseName(),
		}}
	}
	return nil
}

func checkAllowedCharacters(filename string) []CheckResult {
	if match, cleanName := findNotAllowedCharacters(filename); match != "" {
		return []CheckResult{{
			Identifier: "filename_characters",
			Passed:     false,
			Severity:   "warning",
			Warning:    fmt.Sprintf("disallowed character found: %s", match),
			Expected:   cleanName,
			Actual:     filename,
		}}
	}
	return nil
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

func checkCharacterSequences(filename string) []CheckResult {
	if match, cleanName := findCharacterSequences(filename); match != "" {
		return []CheckResult{{
			Identifier: "filename_sequences",
			Passed:     false,
			Severity:   "warning",
			Warning:    fmt.Sprintf("disallowed character sequence found: %s", match),
			Expected:   cleanName,
			Actual:     filename,
		}}
	}
	return nil
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

func checkYearMissing(meta *metadata.Metadata) []CheckResult {
	if meta.Year == 0 && !meta.IsTV {
		return []CheckResult{{
			Identifier: "filename_year_missing",
			Passed:     false,
			Severity:   "warning",
			Warning:    "year is missing for this Movie",
		}}
	}
	return nil
}

func checkYearRedundant(meta *metadata.Metadata) []CheckResult {
	if meta.Year > 0 && meta.Season > 1900 {
		return []CheckResult{{
			Identifier: "filename_year_redundant",
			Passed:     false,
			Severity:   "info",
			Warning:    fmt.Sprintf("redundant Year: The Season (%d) already indicates the year", meta.Season),
		}}
	}
	return nil
}

func checkStreamingService(meta *metadata.Metadata) []CheckResult {
	isWeb := strings.Contains(meta.Source, "WEB")
	if isWeb && meta.Service == "" {
		return []CheckResult{{
			Identifier: "filename_streaming",
			Passed:     false,
			Severity:   "warning",
			Warning:    "Streaming Service Tag is missing for WEB source",
		}}
	} else if !isWeb && meta.Service != "" {
		return []CheckResult{{
			Identifier: "filename_streaming",
			Passed:     false,
			Severity:   "info",
			Warning:    "Streaming Service Tag is not supported for non-WEB source",
		}}
	}
	return nil
}

func checkTvSpecial(meta *metadata.Metadata) []CheckResult {
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

			return []CheckResult{{
				Identifier: "filename_tv_special",
				Passed:     false,
				Severity:   "warning",
				Warning:    fmt.Sprintf("%s is missing for TV Special", warning),
			}}
		}
	}
	return nil
}

package checks

import (
	"fmt"
	"regexp"
	"strings"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
)

// RunFilenameChecks performs checks on the filename and metadata.
func RunFilenameChecks(name string, meta *metadata.Metadata) []CheckResult {
	var results []CheckResult

	if config.IsCheckEnabled(config.CheckFilenameGenerationMismatch) {
		results = append(results, checkNameMismatch(name, meta)...)
	}

	if config.IsCheckEnabled(config.CheckFilenameCharacters) {
		results = append(results, checkAllowedCharacters(name)...)
	}

	if config.IsCheckEnabled(config.CheckFilenameSequences) {
		results = append(results, checkCharacterSequences(name)...)
	}

	if config.IsCheckEnabled(config.CheckFilenameYearMissing) {
		results = append(results, checkYearMissing(meta)...)
	}

	if config.IsCheckEnabled(config.CheckFilenameYearRedundant) {
		results = append(results, checkYearRedundant(meta)...)
	}

	if config.IsCheckEnabled(config.CheckFilenameStreaming) {
		results = append(results, checkStreamingService(meta)...)
	}

	if config.IsCheckEnabled(config.CheckFilenameTVSpecial) {
		results = append(results, checkTvSpecial(meta)...)
	}

	return results
}

func checkNameMismatch(name string, meta *metadata.Metadata) []CheckResult {
	if name != meta.GetReleaseName() {
		return []CheckResult{{
			Identifier: config.CheckFilenameGenerationMismatch,
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
			Identifier: config.CheckFilenameCharacters,
			Passed:     false,
			Severity:   "warning",
			Warning:    "disallowed character found: " + match,
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
			Identifier: config.CheckFilenameSequences,
			Passed:     false,
			Severity:   "warning",
			Warning:    "disallowed character sequence found: " + match,
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
			Identifier: config.CheckFilenameYearMissing,
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
			Identifier: config.CheckFilenameYearRedundant,
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
			Identifier: config.CheckFilenameStreaming,
			Passed:     false,
			Severity:   "warning",
			Warning:    "Streaming Service Tag is missing for WEB source",
		}}
	} else if !isWeb && meta.Service != "" {
		return []CheckResult{{
			Identifier: config.CheckFilenameStreaming,
			Passed:     false,
			Severity:   "info",
			Warning:    "Streaming Service Tag is not supported for non-WEB source",
		}}
	}

	return nil
}

func checkTvSpecial(meta *metadata.Metadata) []CheckResult {
	if !meta.IsTV || meta.Season != 0 {
		return nil
	}

	if meta.Date != "" && len(meta.EpisodeTitles) > 0 {
		return nil
	}

	warning := "Date and Episode Title"
	if meta.Date == "" && len(meta.EpisodeTitles) > 0 {
		warning = "Date"
	} else if meta.Date != "" && len(meta.EpisodeTitles) == 0 {
		warning = "Episode Title"
	}

	return []CheckResult{{
		Identifier: config.CheckFilenameTVSpecial,
		Passed:     false,
		Severity:   "warning",
		Warning:    warning + " is missing for TV Special",
	}}
}

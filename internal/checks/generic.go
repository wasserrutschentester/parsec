package checks

import (
	"fmt"
	"strings"

	"codeberg.org/n0ne/parsec/internal/config"
	"codeberg.org/n0ne/parsec/internal/metadata"
)

func RunGenericChecks(meta *metadata.Metadata) []CheckResult {
	var results []CheckResult
	results = append(results, CheckYear(meta)...)

	if config.IsCheckEnabled("generic_streaming") {
		results = append(results, CheckStreaming(meta)...)
	}
	if config.IsCheckEnabled("generic_tv_special") {
		results = append(results, CheckTvSpecial(meta)...)
	}
	return results
}

func CheckYear(meta *metadata.Metadata) []CheckResult {
	var results []CheckResult

	if config.IsCheckEnabled("generic_year_missing") {
		res := CheckResult{
			Identifier:  "generic_year_missing",
			Description: "Year is missing for this Movie",
			Passed:      true,
		}
		if meta.Year == 0 && !meta.IsTV {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = "year is missing for this Movie"
		}
		results = append(results, res)
	}

	if config.IsCheckEnabled("generic_year_redundant") {
		res := CheckResult{
			Identifier:  "generic_year_redundant",
			Description: "Redundant Year: The Season already indicates the year",
			Passed:      true,
		}
		if meta.Year > 0 && meta.Season > 1900 {
			res.Passed = false
			res.Severity = "info"
			res.Warning = fmt.Sprintf("redundant Year: The Season (%d) already indicates the year", meta.Season)
		}
		results = append(results, res)
	}

	return results
}

func CheckStreaming(meta *metadata.Metadata) []CheckResult {
	res := CheckResult{
		Identifier:  "generic_streaming",
		Description: "Streaming Service Tag for WEB source",
		Passed:      true,
	}

	isWeb := strings.Contains(meta.Source, "WEB")
	if isWeb && meta.Service == "" {
		res.Passed = false
		res.Severity = "warning"
		res.Warning = "Streaming Service Tag is missing for WEB source"
	}

	if !isWeb && meta.Service != "" {
		res.Passed = false
		res.Severity = "info"
		res.Warning = "Streaming Service Tag is not supported for non-WEB source"
	}

	return []CheckResult{res}
}

func CheckTvSpecial(meta *metadata.Metadata) []CheckResult {
	res := CheckResult{
		Identifier:  "generic_tv_special",
		Description: "Date and Episode Title for TV Specials",
		Passed:      true,
	}

	if meta.IsTV && meta.Season == 0 {
		if meta.Date == "" {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = "Date is missing for TV Special"
		} else if meta.EpisodeTitle == "" {
			res.Passed = false
			res.Severity = "warning"
			res.Warning = "Episode Title is missing for TV Special"
		}
	}
	return []CheckResult{res}
}

// Package ui provides components and utilities for the terminal user interface.
package ui

import (
	"fmt"
	"strings"

	"codeberg.org/upPollo/parsec/internal/types"
)

// PrintInteractiveReport displays a formatted report of check results, optionally asking for confirmation before showing details.
func PrintInteractiveReport(report types.CheckReport, unattended bool) {
	if report.Passed {
		Println("\n" + IconCheck + Success.Render(" All systems nominal! The file fits the specification."))
		Println()

		return
	}

	allTracks := getAllTracks(report.Issues)
	sharedWidths := calculateTrackTableWidths(allTracks)

	totalIssues := countIssues(report.Issues)
	Println("\n" + IconCross + Error.Render(fmt.Sprintf(" %d issues found:", totalIssues)))

	for _, group := range report.Issues {
		if !unattended && !ConfirmContinue(fmt.Sprintf("\nDisplay %d %s issues?", len(group.Results), group.Category)) {
			return
		}

		printIssueGroup(group, sharedWidths)
	}
}

func getAllTracks(issues []types.IssueGroup) []types.TrackCheckResult {
	var allTracks []types.TrackCheckResult

	for _, group := range issues {
		for _, res := range group.Results {
			allTracks = append(allTracks, res.Tracks...)
		}
	}

	return allTracks
}

func printIssueGroup(group types.IssueGroup, sharedWidths map[int]int) {
	Println(ReportSection(fmt.Sprintf("%s (%d)", group.Category, len(group.Results))))

	for _, res := range group.Results {
		if res.Severity == "error" {
			PrintError(res.Warning)
		} else {
			PrintWarning(res.Warning)
		}

		if len(res.Tracks) == 0 {
			printUnexpectedDiff(res)
			continue
		}

		Println(formatTrackTable(res.Tracks, sharedWidths))
	}
}

func printUnexpectedDiff(res types.CheckResult) {
	if res.Expected != "" && res.Actual != "" {
		labelE := "Expected"
		labelA := "Actual"

		// Use Official/Parsed for all MDB related checks
		if strings.HasPrefix(res.Identifier, "mdb_") {
			labelE, labelA = "Official", "Parsed"
		} else if res.Identifier == "filename_generation_mismatch" {
			labelE, labelA = "Original", "Generated"
		}

		indent := "   "
		diff := FormatStringDiffAligned(labelE, res.Expected, labelA, res.Actual)
		indentedDiff := indent + strings.ReplaceAll(diff, "\n", "\n"+indent)
		Println(indentedDiff)
	}
}

func countIssues(groups []types.IssueGroup) int {
	count := 0
	for _, group := range groups {
		count += len(group.Results)
	}

	return count
}

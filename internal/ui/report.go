package ui

import (
	"fmt"
	"strings"

	"codeberg.org/upPollo/parsec/internal/types"
)

func PrintInteractiveReport(report types.CheckReport, unattended bool) {
	if report.Passed {
		Println("\n" + IconCheck + Success.Render(" All systems nominal! The file fits the specification."))
		Println()
		return
	}

	// Calculate shared column widths across all tracks to ensure table alignment
	var allTracks []types.TrackCheckResult
	for _, group := range report.Issues {
		for _, res := range group.Results {
			allTracks = append(allTracks, res.Tracks...)
		}
	}
	sharedWidths := CalculateTrackTableWidths(allTracks)

	totalIssues := CountIssues(report.Issues)
	Println("\n" + IconCross + Error.Render(fmt.Sprintf(" %d issues found:", totalIssues)))
	for _, group := range report.Issues {
		count := len(group.Results)
		if !unattended {
			if !ConfirmContinue(fmt.Sprintf("\nDisplay %d %s issues?", count, group.Category)) {
				return
			}
		}

		Println(ReportSection(fmt.Sprintf("%s (%d)", group.Category, count)))
		for _, res := range group.Results {
			switch res.Severity {
			case "error":
				PrintError(res.Warning)
			default:
				PrintWarning(res.Warning)
			}
			if len(res.Tracks) == 0 {
				printUnexpectedDiff(res)
				continue
			}

			Println(FormatTrackTable(res.Tracks, sharedWidths))
		}
	}
	Println()
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

func CountIssues(groups []types.IssueGroup) int {
	count := 0
	for _, group := range groups {
		count += len(group.Results)
	}
	return count
}

// Package ui provides components and utilities for the terminal user interface.
package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"

	"codeberg.org/upPollo/parsec/internal/types"
)

// PrintInteractiveReport displays a formatted report of check results, optionally asking for confirmation before showing details.
func PrintInteractiveReport(report types.CheckReport, unattended bool) {
	if report.Passed {
		Println("\n" + IconCheck + Success.Render(" All systems nominal! The file fits the specification."))

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

	if group.Category == "MATROSKA" {
		printMatroskaGroup(group, sharedWidths)
	} else {
		for _, res := range group.Results {
			printSingleResult(res, sharedWidths)
		}
	}
}

func printMatroskaGroup(group types.IssueGroup, sharedWidths map[int]int) {
	subGroupNames := []string{"Container/Audio", "Tracks", "Subtitles", "Chapters"}
	groupedResults := make(map[string][]types.CheckResult)

	for _, res := range group.Results {
		subGrp := getMatroskaSubGroupName(res.Identifier)
		groupedResults[subGrp] = append(groupedResults[subGrp], res)
	}

	for _, subGrp := range subGroupNames {
		results := groupedResults[subGrp]
		if len(results) == 0 {
			continue
		}

		width, _, _ := term.GetSize(os.Stdout.Fd())

		if width <= 0 {
			width = 80
		}

		if width > 100 {
			width = 100
		}

		prefix := "─── "
		headerText := subGrp
		prefixRunes := utf8.RuneCountInString(prefix)
		headerRunes := utf8.RuneCountInString(headerText)

		suffixLen := max(0, width-prefixRunes-headerRunes-1)
		divider := prefix + headerText + " " + strings.Repeat("─", suffixLen)

		subHeader := lipgloss.NewStyle().Bold(true).Foreground(purple).Render(divider)

		Println()
		Println(subHeader)
		Println()

		for _, res := range results {
			printSingleResult(res, sharedWidths)
		}
	}
}

func printSingleResult(res types.CheckResult, sharedWidths map[int]int) {
	if res.Severity == "error" {
		PrintError(res.Warning)
	} else {
		PrintWarning(res.Warning)
	}

	if res.Table != nil {
		Println(DataTable(res.Table.Headers, res.Table.Rows))

		return
	}

	if len(res.Tracks) == 0 {
		printUnexpectedDiff(res)

		return
	}

	if isListTrackReport(res.Identifier) {
		printListTrackReport(res)

		return
	}

	Println(formatTrackTable(res.Tracks, sharedWidths))
}

var matroskaSubGroups = map[string]string{
	"matroska_language_tag":        "Tracks",
	"matroska_multi_lang":          "Tracks",
	"matroska_name_quality":        "Tracks",
	"matroska_name_codecs":         "Tracks",
	"matroska_name_redundant_lang": "Tracks",
	"matroska_original_language":   "Tracks",
	"matroska_duplicate_tracks":    "Tracks",
	"matroska_name_keywords":       "Tracks",
	"matroska_default_flags":       "Tracks",
	"matroska_track_order":         "Tracks",
	"matroska_track_delay":         "Tracks",

	"matroska_subtitle_format":          "Subtitles",
	"matroska_subtitle_fonts":           "Subtitles",
	"matroska_subtitle_inline_fonts":    "Subtitles",
	"matroska_srt_validation":           "Subtitles",
	"matroska_ass_script_info":          "Subtitles",
	"matroska_ass_styles":               "Subtitles",
	"matroska_ass_events":               "Subtitles",
	"matroska_zlib_compression":         "Subtitles",
	"matroska_unused_fonts":             "Subtitles",
	"matroska_font_filename_compliance": "Subtitles",

	"matroska_chapters_start_non_zero":     "Chapters",
	"matroska_chapters_non_monotonic":      "Chapters",
	"matroska_chapters_duplicate":          "Chapters",
	"matroska_chapters_too_close":          "Chapters",
	"matroska_chapters_exceed_duration":    "Chapters",
	"matroska_chapters_name_hygiene":       "Chapters",
	"matroska_chapters_language_hygiene":   "Chapters",
	"matroska_chapters_keyframe_alignment": "Chapters",
}

func getMatroskaSubGroupName(id string) string {
	if subGrp, ok := matroskaSubGroups[id]; ok {
		return subGrp
	}

	return "Container/Audio"
}

func printListTrackReport(res types.CheckResult) {
	printListTrackReportWithIndent(res.Identifier, res.Tracks, "      ")
}

func printListTrackReportWithIndent(identifier string, tracks []types.TrackCheckResult, indent string) {
	for _, t := range tracks {
		header := fmt.Sprintf("%sTrack %s (%s/%s)", indent, t.ID, t.Type, t.Codec)
		if t.Language != "" {
			header += fmt.Sprintf(" [%s]", t.Language)
		}

		Println(Muted.Render(header + ":"))

		for line := range strings.SplitSeq(t.Warning, "\n") {
			if identifier == "matroska_srt_validation" {
				if !strings.HasPrefix(line, "  ") && strings.HasSuffix(line, ":") {
					Println(indent + Warning.Render(line))
				} else {
					Println(indent + "  - " + strings.TrimSpace(line))
				}
			} else {
				Println(indent + "- " + line)
			}
		}
	}
}

func isListTrackReport(id string) bool {
	return id == "matroska_ass_styles" || id == "matroska_ass_events" || id == "matroska_ass_script_info" || id == "matroska_srt_validation"
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

type issueKey struct {
	category   string
	identifier string
	warning    string
}

type fileDetail struct {
	fileName string
	expected string
	actual   string
	tracks   []types.TrackCheckResult
	table    *types.TableData
}

type aggIssue struct {
	key      issueKey
	severity string
	details  []fileDetail
}

// PrintAggregatedSummary displays a condensed report summarizing common batch issues and grouping outliers.
func PrintAggregatedSummary(reports []types.CheckReport, unattended bool) {
	totalFiles := len(reports)
	passedFiles := 0
	failedFiles := 0

	for _, r := range reports {
		if r.Passed {
			passedFiles++
		} else {
			failedFiles++
		}
	}

	Println(Info.Render("\n.: INTEGRITY VERIFICATION SUMMARY :."))
	Println(fmt.Sprintf("Checked %d files: %d passed, %d have issues.", totalFiles, passedFiles, failedFiles))

	if failedFiles == 0 {
		Println("\n" + IconCheck + Success.Render(" All systems nominal! All files fit the specification."))

		return
	}

	keysOrder, aggIssuesMap := groupIssues(reports)

	// An issue is systemic if it affects >= 60% of checked files
	systemicThreshold := max(int(float64(totalFiles)*0.6), 2)

	var (
		systemicIssues []*aggIssue
		outlierIssues  []*aggIssue
	)

	for _, k := range keysOrder {
		agg := aggIssuesMap[k]
		if len(agg.details) >= systemicThreshold {
			systemicIssues = append(systemicIssues, agg)
		} else {
			outlierIssues = append(outlierIssues, agg)
		}
	}

	printSystemicIssues(systemicIssues, totalFiles)
	promptForFirstAffectedReport(reports, systemicIssues, outlierIssues, unattended)
	printOutlierIssues(outlierIssues, totalFiles, unattended)
}

func promptForFirstAffectedReport(reports []types.CheckReport, systemicIssues, outlierIssues []*aggIssue, unattended bool) {
	if len(systemicIssues) == 0 {
		return
	}

	first := findFirstAffectedReport(reports, systemicIssues, outlierIssues)
	if first == nil {
		return
	}

	baseName := getBaseName(first.File)

	if unattended {
		Println("\nPrinting detailed report for representative file: " + baseName)
		PrintInteractiveReport(*first, true)

		return
	}

	msg := fmt.Sprintf("\nDisplay the detailed report for (%s)?", baseName)

	if ConfirmContinue(msg) {
		PrintInteractiveReport(*first, true)
	}
}

func findFirstAffectedReport(reports []types.CheckReport, systemicIssues, outlierIssues []*aggIssue) *types.CheckReport {
	fileHasSystemic := make(map[string]bool)
	fileHasOutlier := make(map[string]bool)

	for _, agg := range systemicIssues {
		for _, detail := range agg.details {
			fileHasSystemic[detail.fileName] = true
		}
	}

	for _, agg := range outlierIssues {
		for _, detail := range agg.details {
			fileHasOutlier[detail.fileName] = true
		}
	}

	// First pass: find a file with systemic issues and no outliers
	for _, r := range reports {
		baseName := getBaseName(r.File)
		if fileHasSystemic[baseName] && !fileHasOutlier[baseName] {
			return &r
		}
	}

	// Second pass fallback: find first file with systemic issues
	for _, r := range reports {
		baseName := getBaseName(r.File)
		if fileHasSystemic[baseName] {
			return &r
		}
	}

	return nil
}

func groupIssues(reports []types.CheckReport) ([]issueKey, map[issueKey]*aggIssue) {
	aggIssuesMap := make(map[issueKey]*aggIssue)

	var keysOrder []issueKey

	for _, r := range reports {
		baseName := getBaseName(r.File)
		for _, group := range r.Issues {
			for _, res := range group.Results {
				k := issueKey{
					category:   group.Category,
					identifier: res.Identifier,
					warning:    res.Warning,
				}

				detail := fileDetail{
					fileName: baseName,
					expected: res.Expected,
					actual:   res.Actual,
					tracks:   res.Tracks,
					table:    res.Table,
				}

				if agg, exists := aggIssuesMap[k]; exists {
					agg.details = append(agg.details, detail)
				} else {
					agg = &aggIssue{
						key:      k,
						severity: res.Severity,
						details:  []fileDetail{detail},
					}
					aggIssuesMap[k] = agg
					keysOrder = append(keysOrder, k)
				}
			}
		}
	}

	return keysOrder, aggIssuesMap
}

func printSystemicIssues(issues []*aggIssue, totalFiles int) {
	if len(issues) == 0 {
		return
	}

	Println("\n" + Header.Render("SYSTEMIC BATCH ISSUES"))

	for _, agg := range issues {
		msg := fmt.Sprintf("[%s] %s (Affects %d/%d files)", agg.key.category, agg.key.warning, len(agg.details), totalFiles)
		if agg.severity == "error" {
			PrintError(msg)
		} else {
			PrintWarning(msg)
		}
	}
}

func printOutlierIssues(issues []*aggIssue, totalFiles int, unattended bool) {
	if len(issues) == 0 {
		return
	}

	if !unattended {
		msg := fmt.Sprintf("\nDisplay %d file-specific outliers?", len(issues))

		if !ConfirmContinue(msg) {
			return
		}
	}

	Println("\n" + Header.Render("FILE-SPECIFIC ANOMALIES / OUTLIERS"))

	for _, agg := range issues {
		msg := fmt.Sprintf("[%s] %s (Affects %d/%d files)", agg.key.category, agg.key.warning, len(agg.details), totalFiles)
		if agg.severity == "error" {
			PrintError(msg)
		} else {
			PrintWarning(msg)
		}

		for _, detail := range agg.details {
			Println(fmt.Sprintf("    • %s:", detail.fileName))

			if detail.table != nil {
				tableStr := DataTable(detail.table.Headers, detail.table.Rows)
				indentedTable := "        " + strings.ReplaceAll(tableStr, "\n", "\n        ")
				Println(indentedTable)
			} else {
				// Print diff details if any
				printUnexpectedDiffIndented(agg.key.identifier, detail.expected, detail.actual, "        ")

				// Print tracks table if any
				printTracksDetailsIndented(agg.key.identifier, detail.tracks)
			}
		}
	}
}

func printTracksDetailsIndented(identifier string, tracks []types.TrackCheckResult) {
	if len(tracks) == 0 {
		return
	}

	if isListTrackReport(identifier) {
		printListTrackReportWithIndent(identifier, tracks, "          ")
	} else {
		widths := calculateTrackTableWidths(tracks)
		tableStr := formatTrackTable(tracks, widths)
		indentedTable := "        " + strings.ReplaceAll(tableStr, "\n", "\n        ")
		Println(indentedTable)
	}
}

func printUnexpectedDiffIndented(identifier, expected, actual, indent string) {
	if expected != "" && actual != "" {
		labelE := "Expected"
		labelA := "Actual"

		if strings.HasPrefix(identifier, "mdb_") {
			labelE, labelA = "Official", "Parsed"
		} else if identifier == "filename_generation_mismatch" {
			labelE, labelA = "Original", "Generated"
		}

		diff := FormatStringDiffAligned(labelE, expected, labelA, actual)
		indentedDiff := indent + strings.ReplaceAll(diff, "\n", "\n"+indent)
		Println(indentedDiff)
	}
}

func getBaseName(path string) string {
	base := filepath.Base(path)
	if idx := strings.LastIndex(base, "."); idx != -1 {
		return base[:idx]
	}

	return base
}

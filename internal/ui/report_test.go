package ui

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"

	"codeberg.org/upPollo/parsec/internal/types"
)

//nolint:funlen,paralleltest // test reports initialization is long and redirects global stdout/stderr
func TestPrintAggregatedSummary(t *testing.T) {
	// Create sample check reports
	reports := []types.CheckReport{
		{
			File:   "/data/S03E01.mkv",
			Passed: false,
			Issues: []types.IssueGroup{
				{
					Category: "MDB",
					Results: []types.CheckResult{
						{
							Identifier: "mdb_missing_tvdb",
							Passed:     false,
							Severity:   "warning",
							Warning:    "Missing tag: TVDB ID",
						},
						{
							Identifier: "mdb_casing_mismatch",
							Passed:     false,
							Severity:   "warning",
							Warning:    "Title casing mismatch",
						},
					},
				},
			},
		},
		{
			File:   "/data/S03E02.mkv",
			Passed: false,
			Issues: []types.IssueGroup{
				{
					Category: "MDB",
					Results: []types.CheckResult{
						{
							Identifier: "mdb_missing_tvdb",
							Passed:     false,
							Severity:   "warning",
							Warning:    "Missing tag: TVDB ID",
						},
						{
							Identifier: "mdb_casing_mismatch",
							Passed:     false,
							Severity:   "warning",
							Warning:    "Title casing mismatch",
						},
					},
				},
			},
		},
		{
			File:   "/data/S03E03.mkv",
			Passed: false,
			Issues: []types.IssueGroup{
				{
					Category: "MDB",
					Results: []types.CheckResult{
						{
							Identifier: "mdb_missing_tvdb",
							Passed:     false,
							Severity:   "warning",
							Warning:    "Missing tag: TVDB ID",
						},
						{
							Identifier: "mdb_casing_mismatch",
							Passed:     false,
							Severity:   "warning",
							Warning:    "Title casing mismatch",
						},
					},
				},
				{
					Category: "MATROSKA",
					Results: []types.CheckResult{
						{
							Identifier: "matroska_track_lang",
							Passed:     false,
							Severity:   "error",
							Warning:    "Track 3 language missing",
							Expected:   "eng",
							Actual:     "und",
						},
					},
				},
			},
		},
	}

	// Capture stdout and stderr
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	os.Stdout = w
	os.Stderr = w

	oldWriter := lipgloss.Writer
	lipgloss.Writer = colorprofile.NewWriter(w, os.Environ())

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		lipgloss.Writer = oldWriter
	}()

	PrintAggregatedSummary(reports, true)

	_ = w.Close()

	var buf bytes.Buffer

	_, _ = io.Copy(&buf, r)
	out := buf.String()

	// Verify that systemic issues are displayed
	if !strings.Contains(out, "SYSTEMIC BATCH ISSUES") {
		t.Error("Expected output to contain 'SYSTEMIC BATCH ISSUES' header")
	}

	if !strings.Contains(out, "Missing tag: TVDB ID (Affects 3/3 files)") {
		t.Error("Expected systemic issue 'Missing tag: TVDB ID' to be summarized")
	}

	if !strings.Contains(out, "Title casing mismatch (Affects 3/3 files)") {
		t.Error("Expected systemic issue 'Title casing mismatch' to be summarized")
	}

	// Verify that the chosen representative file is printed in unattended mode
	if !strings.Contains(out, "Printing detailed report for representative file: S03E01") {
		t.Error("Expected output to notify which representative file was chosen in unattended mode")
	}

	// Verify that outlier issues are displayed
	if !strings.Contains(out, "FILE-SPECIFIC ANOMALIES / OUTLIERS") {
		t.Error("Expected output to contain 'FILE-SPECIFIC ANOMALIES / OUTLIERS' header")
	}

	if !strings.Contains(out, "Track 3 language missing (Affects 1/3 files)") {
		t.Error("Expected outlier issue 'Track 3 language missing' to be listed")
	}

	if !strings.Contains(out, "• S03E03:") {
		t.Error("Expected outlier section to list the specific affected file 'S03E03'")
	}

	if !strings.Contains(out, "Expected") || !strings.Contains(out, "Actual") {
		t.Error("Expected outlier section to show unexpected diff expected/actual labels")
	}
}

//nolint:funlen,paralleltest // test reports initialization is long and redirects global stdout/stderr
func TestPrintAggregatedSummaryPrioritizeNoOutliers(t *testing.T) {
	// Create sample check reports
	// reports[0] (S03E01) has a systemic issue AND an outlier issue
	// reports[1] (S03E02) has only the systemic issues (no outliers)
	reports := []types.CheckReport{
		{
			File:   "/data/S03E01.mkv",
			Passed: false,
			Issues: []types.IssueGroup{
				{
					Category: "MDB",
					Results: []types.CheckResult{
						{
							Identifier: "mdb_missing_tvdb",
							Passed:     false,
							Severity:   "warning",
							Warning:    "Missing tag: TVDB ID",
						},
					},
				},
				{
					Category: "MATROSKA",
					Results: []types.CheckResult{
						{
							Identifier: "matroska_track_lang",
							Passed:     false,
							Severity:   "error",
							Warning:    "Track 3 language missing",
							Expected:   "eng",
							Actual:     "und",
						},
					},
				},
			},
		},
		{
			File:   "/data/S03E02.mkv",
			Passed: false,
			Issues: []types.IssueGroup{
				{
					Category: "MDB",
					Results: []types.CheckResult{
						{
							Identifier: "mdb_missing_tvdb",
							Passed:     false,
							Severity:   "warning",
							Warning:    "Missing tag: TVDB ID",
						},
					},
				},
			},
		},
		{
			File:   "/data/S03E03.mkv",
			Passed: false,
			Issues: []types.IssueGroup{
				{
					Category: "MDB",
					Results: []types.CheckResult{
						{
							Identifier: "mdb_missing_tvdb",
							Passed:     false,
							Severity:   "warning",
							Warning:    "Missing tag: TVDB ID",
						},
					},
				},
			},
		},
	}

	// Capture stdout/stderr
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	os.Stdout = w
	os.Stderr = w

	oldWriter := lipgloss.Writer
	lipgloss.Writer = colorprofile.NewWriter(w, os.Environ())

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		lipgloss.Writer = oldWriter
	}()

	PrintAggregatedSummary(reports, true)

	_ = w.Close()

	var buf bytes.Buffer

	_, _ = io.Copy(&buf, r)

	out := buf.String()

	// Verify that the chosen representative file is S03E02 instead of S03E01
	// Because S03E02 has no outliers whereas S03E01 has an outlier.
	if !strings.Contains(out, "Printing detailed report for representative file: S03E02") {
		t.Error("Expected output to prioritize S03E02 as the representative file because it has no outliers")
	}
}

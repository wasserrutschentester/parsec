package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/types"
)

func TestFilterFailedReports(t *testing.T) {
	t.Parallel()

	reports := []types.CheckReport{
		{
			File:   "/path/to/ok.mkv",
			Passed: true,
		},
		{
			File:   "/path/to/bad_runtime.mkv",
			Passed: false,
			Issues: []types.IssueGroup{
				{
					Category: "MDB",
					Results: []types.CheckResult{
						{Identifier: "mdb_runtime", Passed: false},
					},
				},
			},
		},
		{
			File:   "/path/to/bad_filename.mkv",
			Passed: false,
			Issues: []types.IssueGroup{
				{
					Category: "FILENAME",
					Results: []types.CheckResult{
						{Identifier: "filename_characters", Passed: false},
					},
				},
			},
		},
	}

	allFailed := filterFailedReports(reports, "")
	if len(allFailed) != 2 || allFailed[0] != "/path/to/bad_runtime.mkv" || allFailed[1] != "/path/to/bad_filename.mkv" {
		t.Errorf("filterFailedReports(reports, \"\") = %v, want 2 files", allFailed)
	}

	runtimeFailed := filterFailedReports(reports, "mdb_runtime")
	if len(runtimeFailed) != 1 || runtimeFailed[0] != "/path/to/bad_runtime.mkv" {
		t.Errorf("filterFailedReports(reports, \"mdb_runtime\") = %v, want [bad_runtime.mkv]", runtimeFailed)
	}

	nonExistent := filterFailedReports(reports, "non_existent")
	if len(nonExistent) != 0 {
		t.Errorf("filterFailedReports(reports, \"non_existent\") = %v, want []", nonExistent)
	}
}

func TestCompleteCheckIdentifiers(t *testing.T) {
	t.Parallel()

	results, directive := completeCheckIdentifiers(nil, nil, "")
	if len(results) != len(config.AllChecks) {
		t.Errorf("completeCheckIdentifiers() count = %d, want %d", len(results), len(config.AllChecks))
	}

	if !slices.IsSorted(results) {
		t.Errorf("completeCheckIdentifiers() results not sorted")
	}

	if directive != 4 { // cobra.ShellCompDirectiveNoFileComp = 4
		t.Errorf("completeCheckIdentifiers() directive = %v, want ShellCompDirectiveNoFileComp", directive)
	}
}

//nolint:paralleltest // modifies global flag state
func TestMoveFailedFiles(t *testing.T) {
	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "fail.mkv")
	destDir := filepath.Join(tempDir, "quarantine")

	if err := os.WriteFile(srcFile, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("failed to create srcFile: %v", err)
	}

	reports := []types.CheckReport{
		{
			File:   srcFile,
			Passed: false,
			Issues: []types.IssueGroup{
				{
					Category: "MDB",
					Results: []types.CheckResult{
						{Identifier: "mdb_runtime", Passed: false},
					},
				},
			},
		},
	}

	unattendedFlag = true
	defer func() { unattendedFlag = false }()

	if err := moveFailedFiles(reports, destDir, "mdb_runtime"); err != nil {
		t.Fatalf("moveFailedFiles failed: %v", err)
	}

	destFile := filepath.Join(destDir, "fail.mkv")
	if _, err := os.Stat(destFile); err != nil {
		t.Errorf("destFile was not moved to quarantine: %v", err)
	}

	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Errorf("srcFile still exists in original location")
	}
}

package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

var errTestOther = errors.New("boom")

// Regression test: a failure identifying one file in a multi-file batch
// (including a bad answer to the interactive disambiguation prompt) must not
// abort the rest of the batch. batchIdentifyError should only report failure
// when every file failed.
func TestBatchIdentifyError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		errs    []error
		wantErr bool
	}{
		{name: "no files", errs: nil, wantErr: false},
		{name: "all succeeded", errs: []error{nil, nil}, wantErr: false},
		{name: "one failure among successes is still a failure", errs: []error{nil, errTestOther, nil}, wantErr: true},
		{name: "all failed", errs: []error{errTestOther, errSearch}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := batchIdentifyError(tt.errs)
			if (err != nil) != tt.wantErr {
				t.Errorf("batchIdentifyError(%v) = %v, wantErr %v", tt.errs, err, tt.wantErr)
			}
		})
	}
}

//nolint:paralleltest // modifies global flag state
func TestMoveTaggedFiles(t *testing.T) {
	tests := []struct {
		name      string
		dryRun    bool
		wantMoved bool
	}{
		{name: "standard move", dryRun: false, wantMoved: true},
		{name: "dry run", dryRun: true, wantMoved: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			srcFile := filepath.Join(tempDir, "tagged.mkv")
			destDir := filepath.Join(tempDir, "completed")

			if err := os.WriteFile(srcFile, []byte("dummy"), 0o644); err != nil {
				t.Fatalf("failed to create srcFile: %v", err)
			}

			dryRunFlag = tt.dryRun
			unattendedFlag = !tt.dryRun

			defer func() {
				dryRunFlag = false
				unattendedFlag = false
			}()

			if err := moveTaggedFiles([]string{srcFile}, destDir); err != nil {
				t.Fatalf("moveTaggedFiles failed: %v", err)
			}

			destFile := filepath.Join(destDir, "tagged.mkv")
			if _, err := os.Stat(destFile); (err == nil) != tt.wantMoved {
				t.Errorf("destFile exists = %v, want %v", err == nil, tt.wantMoved)
			}

			if _, err := os.Stat(srcFile); (err == nil) == tt.wantMoved {
				t.Errorf("srcFile exists = %v, want %v", err == nil, !tt.wantMoved)
			}
		})
	}
}

func TestMoveTaggedFilesEmpty(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	destDir := filepath.Join(tempDir, "completed")

	if err := moveTaggedFiles(nil, destDir); err != nil {
		t.Fatalf("moveTaggedFiles(nil) failed: %v", err)
	}

	if _, err := os.Stat(destDir); !os.IsNotExist(err) {
		t.Errorf("destDir was created when no files were tagged")
	}
}

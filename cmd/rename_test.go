package cmd

import (
	"path/filepath"
	"testing"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
)

type renameNewPathTestCase struct {
	name              string
	filePath          string
	seasonPackFlag    bool
	releaseFolderFlag bool
	wantDestDir       string
}

func getRenameNewPathTestCases(seasonPackName, newNameBase string) []renameNewPathTestCase {
	return []renameNewPathTestCase{
		{
			name:              "No flags, simple file",
			filePath:          "/home/user/downloads/show.mkv",
			seasonPackFlag:    false,
			releaseFolderFlag: false,
			wantDestDir:       "/home/user/downloads",
		},
		{
			name:              "Season pack only, first run",
			filePath:          "/home/user/downloads/show.mkv",
			seasonPackFlag:    true,
			releaseFolderFlag: false,
			wantDestDir:       filepath.Join("/home/user/downloads", seasonPackName),
		},
		{
			name:              "Season pack only, second run (already inside season pack)",
			filePath:          filepath.Join("/home/user/downloads", seasonPackName, "show.mkv"),
			seasonPackFlag:    true,
			releaseFolderFlag: false,
			wantDestDir:       filepath.Join("/home/user/downloads", seasonPackName),
		},
		{
			name:              "Release folder only, first run",
			filePath:          "/home/user/downloads/show.mkv",
			seasonPackFlag:    false,
			releaseFolderFlag: true,
			wantDestDir:       filepath.Join("/home/user/downloads", newNameBase),
		},
		{
			name:              "Release folder only, second run (already inside release folder)",
			filePath:          filepath.Join("/home/user/downloads", newNameBase, "show.mkv"),
			seasonPackFlag:    false,
			releaseFolderFlag: true,
			wantDestDir:       filepath.Join("/home/user/downloads", newNameBase),
		},
		{
			name:              "Both flags, first run",
			filePath:          "/home/user/downloads/show.mkv",
			seasonPackFlag:    true,
			releaseFolderFlag: true,
			wantDestDir:       filepath.Join("/home/user/downloads", seasonPackName, newNameBase),
		},
		{
			name:              "Both flags, second run (file in release folder, parent is season pack)",
			filePath:          filepath.Join("/home/user/downloads", seasonPackName, newNameBase, "show.mkv"),
			seasonPackFlag:    true,
			releaseFolderFlag: true,
			wantDestDir:       filepath.Join("/home/user/downloads", seasonPackName, newNameBase),
		},
	}
}

func TestRenameNewPathMultipleFlags(t *testing.T) {
	t.Parallel()

	config.InitDefaults()

	meta := &metadata.Metadata{
		Title:    "Show Title",
		Season:   1,
		Episodes: []int{1},
		IsTV:     true,
		Group:    "GRP",
	}

	newNameBase := meta.GetReleaseName()
	seasonPackName := meta.GetSeasonPackName()

	if newNameBase == "" || seasonPackName == "" {
		t.Fatalf("Expected non-empty newNameBase and seasonPackName, got newNameBase=%q, seasonPackName=%q", newNameBase, seasonPackName)
	}

	tests := getRenameNewPathTestCases(seasonPackName, newNameBase)

	for _, tt := range tests {
		oldSeasonPackFlag := seasonPackFlag
		oldReleaseFolderFlag := releaseFolderFlag

		seasonPackFlag = tt.seasonPackFlag
		releaseFolderFlag = tt.releaseFolderFlag

		gotDestDir, gotNewNameBase := renameNewPath(tt.filePath, meta)

		if gotNewNameBase != newNameBase {
			t.Errorf("[%s] gotNewNameBase = %q, want %q", tt.name, gotNewNameBase, newNameBase)
		}

		wantClean := filepath.Clean(tt.wantDestDir)
		gotClean := filepath.Clean(gotDestDir)

		if gotClean != wantClean {
			t.Errorf("[%s] gotDestDir = %q, want %q", tt.name, gotClean, wantClean)
		}

		// Restore flag state after each case since they run sequentially
		seasonPackFlag = oldSeasonPackFlag
		releaseFolderFlag = oldReleaseFolderFlag
	}
}

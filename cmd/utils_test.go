package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandArgs(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "parsec-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create some dummy MKV files with EBML header
	mkvHeader := []byte{0x1A, 0x45, 0xDF, 0xA3}

	mkv1 := filepath.Join(tempDir, "file1.mkv")
	if err := os.WriteFile(mkv1, mkvHeader, 0644); err != nil {
		t.Fatalf("Failed to write mkv1: %v", err)
	}

	mkv2 := filepath.Join(tempDir, "file2.mkv")
	if err := os.WriteFile(mkv2, mkvHeader, 0644); err != nil {
		t.Fatalf("Failed to write mkv2: %v", err)
	}

	// Create a non-MKV file
	txt1 := filepath.Join(tempDir, "file3.txt")
	if err := os.WriteFile(txt1, []byte("not a mkv"), 0644); err != nil {
		t.Fatalf("Failed to write txt1: %v", err)
	}

	// Create a file with .mkv extension but wrong header
	badMkv := filepath.Join(tempDir, "bad.mkv")
	if err := os.WriteFile(badMkv, []byte("fake mkv"), 0644); err != nil {
		t.Fatalf("Failed to write badMkv: %v", err)
	}

	// Create a sub-directory
	subDir := filepath.Join(tempDir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subDir: %v", err)
	}

	mkv3 := filepath.Join(subDir, "file3.mkv")
	if err := os.WriteFile(mkv3, mkvHeader, 0644); err != nil {
		t.Fatalf("Failed to write mkv3: %v", err)
	}

	// Test expansion
	args := []string{tempDir}
	expanded := expandArgs(args)

	// Should find mkv1, mkv2 and mkv3
	if len(expanded) != 3 {
		t.Errorf("Expected 3 files, got %d: %v", len(expanded), expanded)
	}

	found1, found2, found3 := false, false, false
	for _, f := range expanded {
		switch filepath.Base(f) {
		case "file1.mkv":
			found1 = true
		case "file2.mkv":
			found2 = true
		case "file3.mkv":
			found3 = true
		}
	}

	if !found1 || !found2 || !found3 {
		t.Errorf("Did not find all MKV files. Found: %v", expanded)
	}
}

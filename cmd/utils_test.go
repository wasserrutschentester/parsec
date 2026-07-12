package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/config"
)

//nolint:funlen,cyclop // test cases are numerous and involve setup/teardown
func TestExpandArgs(t *testing.T) {
	t.Parallel()

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "parsec-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create some dummy MKV files with EBML header
	mkvHeader := []byte{0x1A, 0x45, 0xDF, 0xA3}

	mkv1 := filepath.Join(tempDir, "file1.mkv")
	if err := os.WriteFile(mkv1, mkvHeader, 0o644); err != nil {
		t.Fatalf("Failed to write mkv1: %v", err)
	}

	mkv2 := filepath.Join(tempDir, "file2.mkv")
	if err := os.WriteFile(mkv2, mkvHeader, 0o644); err != nil {
		t.Fatalf("Failed to write mkv2: %v", err)
	}

	// Create a non-MKV file
	txt1 := filepath.Join(tempDir, "file3.txt")
	if err := os.WriteFile(txt1, []byte("not a mkv"), 0o644); err != nil {
		t.Fatalf("Failed to write txt1: %v", err)
	}

	// Create a file with .mkv extension but wrong header
	badMkv := filepath.Join(tempDir, "bad.mkv")
	if err := os.WriteFile(badMkv, []byte("fake mkv"), 0o644); err != nil {
		t.Fatalf("Failed to write badMkv: %v", err)
	}

	// Create a sub-directory
	subDir := filepath.Join(tempDir, "sub")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create subDir: %v", err)
	}

	mkv3 := filepath.Join(subDir, "file3.mkv")
	if err := os.WriteFile(mkv3, mkvHeader, 0o644); err != nil {
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

func TestCompleteFiles(t *testing.T) {
	t.Parallel()

	tempDir, err := os.MkdirTemp("", "parsec-complete-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test files/directories
	_ = os.WriteFile(filepath.Join(tempDir, "test.mkv"), []byte("test"), 0o644)
	_ = os.WriteFile(filepath.Join(tempDir, "test.json"), []byte("test"), 0o644)
	_ = os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("test"), 0o644)
	_ = os.WriteFile(filepath.Join(tempDir, ".hidden.mkv"), []byte("test"), 0o644)
	_ = os.Mkdir(filepath.Join(tempDir, "subdir"), 0o755)
	_ = os.Mkdir(filepath.Join(tempDir, ".hiddendir"), 0o755)

	assertCompletionsWithoutDot(t, tempDir)
	assertCompletionsWithDot(t, tempDir)
}

func assertCompletionsWithoutDot(t *testing.T, tempDir string) {
	t.Helper()

	comps, _ := completeFiles(tempDir+string(filepath.Separator), "mkv")

	assertContains(t, comps, filepath.Join(tempDir, "test.mkv"), true)
	assertContains(t, comps, filepath.Join(tempDir, "subdir")+string(filepath.Separator), true)
	assertContains(t, comps, filepath.Join(tempDir, "test.txt"), false)
	assertContains(t, comps, filepath.Join(tempDir, ".hidden.mkv"), false)
	assertContains(t, comps, filepath.Join(tempDir, ".hiddendir")+string(filepath.Separator), false)
}

func assertCompletionsWithDot(t *testing.T, tempDir string) {
	t.Helper()

	compsDot, _ := completeFiles(tempDir+string(filepath.Separator)+".", "mkv")

	assertContains(t, compsDot, filepath.Join(tempDir, ".hidden.mkv"), true)
	assertContains(t, compsDot, filepath.Join(tempDir, ".hiddendir")+string(filepath.Separator), true)
}

func assertContains(t *testing.T, list []string, val string, expected bool) {
	t.Helper()

	found := slices.Contains(list, val)

	if found != expected {
		if expected {
			t.Errorf("Expected completions to contain %q, but it did not", val)
		} else {
			t.Errorf("Expected completions NOT to contain %q, but it did", val)
		}
	}
}

func TestStaticFlagCompletions(t *testing.T) {
	t.Parallel()

	t.Run("sources", func(t *testing.T) {
		t.Parallel()

		res, _ := completeSources(nil, nil, "")
		if len(res) == 0 {
			t.Fatal("Expected sources completions to be populated")
		}

		if res[0] != "BluRay\tBlu-ray disc source" {
			t.Errorf("Unexpected first source suggestion: %s", res[0])
		}
	})

	t.Run("hdrs", func(t *testing.T) {
		t.Parallel()

		res, _ := completeHdrs(nil, nil, "")
		if len(res) == 0 {
			t.Fatal("Expected hdrs completions to be populated")
		}

		if res[0] != "DV\tDolby Vision" {
			t.Errorf("Unexpected first HDR suggestion: %s", res[0])
		}
	})

	t.Run("languages", func(t *testing.T) {
		t.Parallel()

		res, _ := completeLanguages(nil, nil, "")
		if len(res) == 0 {
			t.Fatal("Expected languages completions to be populated")
		}

		if res[0] != "ar\tArabic" {
			t.Errorf("Unexpected first language suggestion: %s", res[0])
		}
	})

	t.Run("services", func(t *testing.T) {
		t.Parallel()

		res, _ := completeServices(nil, nil, "")
		if len(res) == 0 {
			t.Fatal("Expected services completions to be populated")
		}

		if res[0] != "AMZN\tAmazon Prime Video" {
			t.Errorf("Unexpected first service suggestion: %s", res[0])
		}
	})
}

//nolint:paralleltest // modifies global viper state
func TestCutEditionsCompletion(t *testing.T) {
	viper.Reset()
	config.InitDefaults()

	// Test default: separator is "."
	res, _ := completeCutEditions(nil, nil, "")
	foundDot := false

	for _, opt := range res {
		if strings.HasPrefix(opt, "3D.HOU\t") {
			foundDot = true

			break
		}
	}

	if !foundDot {
		t.Error("Expected 3D.HOU (with dot separator) in completions by default")
	}

	// Test overridden: separator is " " (space)
	viper.Set("word_separator", " ")

	resSpace, _ := completeCutEditions(nil, nil, "")
	foundSpace := false

	for _, opt := range resSpace {
		if strings.HasPrefix(opt, "3D HOU\t") {
			foundSpace = true

			break
		}
	}

	if !foundSpace {
		t.Error("Expected 3D HOU (with space separator) in completions when word_separator is space")
	}
}

//nolint:paralleltest // modifies global viper state
func TestGroupCompletion(t *testing.T) {
	viper.Reset()
	// Test set
	viper.Set("group", "PAARSEX")

	res, _ := completeGroups(nil, nil, "")
	if len(res) == 0 {
		t.Fatal("Expected group completion to be populated")
	}

	if res[0] != "PAARSEX\tConfigured default group" {
		t.Errorf("Unexpected group suggestion: %s", res[0])
	}
}

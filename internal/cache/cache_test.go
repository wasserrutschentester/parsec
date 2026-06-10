package cache

import (
	"bytes"
	"os"
	"testing"
	"time"
)

func TestCache(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "parsec-test-cache")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	setDir(tempDir)

	defer resetDir()

	key := "test-key"
	data := []byte("test-data")

	// Test Set
	if err := Set(key, data); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Test Get
	cached, err := Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if !bytes.Equal(cached, data) {
		t.Errorf("Expected %s, got %s", data, cached)
	}

	// Test Missing
	if _, err := Get("missing"); err == nil {
		t.Error("Expected error for missing key")
	}

	// Test Expiration (manual mod time change)
	path := getPath(key)

	oldTime := time.Now().Add(-7 * time.Hour)
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	if _, err := Get(key); err == nil {
		t.Error("Expected error for expired key")
	}

	// Test Clear
	_ = Set(key, data)

	clear()

	if _, err := Get(key); err == nil {
		t.Error("Expected error after clear")
	}
}

func TestCleanup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "parsec-test-cleanup")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	setDir(tempDir)

	defer resetDir()

	// Create a fresh file
	keyFresh := "fresh"
	if err := Set(keyFresh, []byte("fresh-data")); err != nil {
		t.Fatal(err)
	}

	// Create an old file
	keyOld := "old"
	if err := Set(keyOld, []byte("old-data")); err != nil {
		t.Fatal(err)
	}

	pathOld := getPath(keyOld)

	oldTime := time.Now().Add(-cacheDuration - time.Hour)
	if err := os.Chtimes(pathOld, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// Run cleanup
	cleanup()

	// Fresh file should still exist
	if _, err := os.Stat(getPath(keyFresh)); err != nil {
		t.Errorf("Fresh file should exist: %v", err)
	}

	// Old file should be gone
	if _, err := os.Stat(pathOld); !os.IsNotExist(err) {
		t.Errorf("Old file should be removed, err: %v", err)
	}
}

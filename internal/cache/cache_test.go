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

	SetDir(tempDir)
	defer ResetDir()

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
	Clear()
	if _, err := Get(key); err == nil {
		t.Error("Expected error after Clear")
	}
}

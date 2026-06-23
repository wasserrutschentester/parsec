package cache

import (
	"bytes"
	"os"
	"testing"
	"time"
)

//nolint:paralleltest // modifies package-level state (cacheDir)
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

	clearCache()

	if _, err := Get(key); err == nil {
		t.Error("Expected error after clear")
	}
}

//nolint:paralleltest // modifies package-level state (cacheDir)
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

//nolint:paralleltest // modifies package-level state (cacheDir, metaDir)
func TestPersistentCache(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "parsec-test-persistent-cache")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	setDir(tempDir)

	defer resetDir()

	key := "test-persistent-key"
	data := []byte("test-persistent-data")
	newKey := "test-persistent-new-key"

	testSetGetPersistent(t, key, data)
	testExpirationAndTouch(t, key, data)
	testPruningAndCleanup(t, key)
	testMoveAndClear(t, key, newKey, data)
}

func testSetGetPersistent(t *testing.T, key string, data []byte) {
	t.Run("SetPersistent", func(t *testing.T) {
		if err := SetPersistent(key, data); err != nil {
			t.Fatalf("SetPersistent failed: %v", err)
		}
	})

	t.Run("GetPersistent", func(t *testing.T) {
		cached, err := GetPersistent(key)
		if err != nil {
			t.Fatalf("GetPersistent failed: %v", err)
		}

		if !bytes.Equal(cached, data) {
			t.Errorf("Expected %s, got %s", data, cached)
		}
	})
}

func testExpirationAndTouch(t *testing.T, key string, data []byte) {
	t.Run("ExpirationUnderGetPersistent", func(t *testing.T) {
		path := getMetaPath(key)
		oldTime := time.Now().Add(-7 * time.Hour)

		if err := os.Chtimes(path, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}

		cachedExpired, err := GetPersistent(key)
		if err != nil {
			t.Fatalf("GetPersistent should not fail on expired modtime: %v", err)
		}

		if !bytes.Equal(cachedExpired, data) {
			t.Errorf("Expected %s, got %s", data, cachedExpired)
		}
	})

	t.Run("GetPersistentTouchesFile", func(t *testing.T) {
		path := getMetaPath(key)
		oldTime := time.Now().Add(-1 * time.Hour)

		if err := os.Chtimes(path, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}

		_, err := GetPersistent(key)
		if err != nil {
			t.Fatal(err)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}

		if time.Since(info.ModTime()) > 5*time.Second {
			t.Errorf("GetPersistent should update modtime to now, got: %v", info.ModTime())
		}
	})
}

func testPruningAndCleanup(t *testing.T, key string) {
	t.Run("CleanupDoesNotRemovePersistent", func(t *testing.T) {
		path := getMetaPath(key)

		cleanup()

		if _, err := os.Stat(path); err != nil {
			t.Errorf("Persistent file should not be removed by cleanup: %v", err)
		}
	})

	t.Run("CleanupRemovesExpiredPersistent", func(t *testing.T) {
		path := getMetaPath(key)
		oldTime := time.Now().Add(-31 * 24 * time.Hour)

		if err := os.Chtimes(path, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}

		removeExpiredMetaFiles()

		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("Persistent file older than 30 days should be removed by removeExpiredMetaFiles")
		}
	})
}

func testMoveAndClear(t *testing.T, key, newKey string, data []byte) {
	t.Run("MovePersistent", func(t *testing.T) {
		path := getMetaPath(key)

		if err := SetPersistent(key, data); err != nil {
			t.Fatalf("SetPersistent failed in MovePersistent pre-requisite: %v", err)
		}

		if err := MovePersistent(key, newKey); err != nil {
			t.Fatalf("MovePersistent failed: %v", err)
		}

		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("Old persistent cache file should not exist after MovePersistent")
		}

		newCached, err := GetPersistent(newKey)
		if err != nil {
			t.Fatalf("GetPersistent failed for moved key: %v", err)
		}

		if !bytes.Equal(newCached, data) {
			t.Errorf("Expected %s, got %s for moved cache", data, newCached)
		}
	})

	t.Run("ClearCacheRemovesMovedPersistent", func(t *testing.T) {
		newPath := getMetaPath(newKey)

		clearCache()

		if _, err := os.Stat(newPath); !os.IsNotExist(err) {
			t.Errorf("Moved persistent file should be removed by clearCache: %v", err)
		}
	})
}

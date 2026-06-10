// Package cache provides a simple file-based cache for API responses.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"codeberg.org/upPollo/parsec/internal/config"
)

var (
	cacheDir string

	errCacheBypassed = errors.New("cache bypassed")
	errCacheExpired  = errors.New("cache expired")
)

const cacheDuration = 6 * time.Hour

func init() {
	resetDir()
	cleanup()
}

func resetDir() {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}

	cacheDir = filepath.Join(dir, "parsec", "api")
}

func setDir(dir string) {
	cacheDir = dir
}

func cleanup() {
	// check if cache directory exists`
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return
	}

	markerPath := filepath.Join(cacheDir, ".last_cleanup")
	if info, err := os.Stat(markerPath); err == nil {
		if time.Since(info.ModTime()) < cacheDuration {
			return
		}
	}

	removeExpiredFiles()

	// Update marker
	_ = os.WriteFile(markerPath, []byte{}, 0o644)
}

func removeExpiredFiles() {
	files, err := os.ReadDir(cacheDir)
	if err != nil {
		return
	}

	for _, file := range files {
		if file.IsDir() || file.Name() == ".last_cleanup" {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		if time.Since(info.ModTime()) > cacheDuration {
			_ = os.Remove(filepath.Join(cacheDir, file.Name()))
		}
	}
}

func clearCache() {
	_ = os.RemoveAll(cacheDir)
}

// Get retrieves data from the cache for the given key.
func Get(key string) ([]byte, error) {
	if config.NoCache {
		return nil, errCacheBypassed
	}

	path := getPath(key)

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat cache file: %w", err)
	}

	if time.Since(info.ModTime()) > cacheDuration {
		_ = os.Remove(path)

		return nil, errCacheExpired
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	return data, nil
}

// Set stores data in the cache for the given key.
func Set(key string, data []byte) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	if err := os.WriteFile(getPath(key), data, 0o644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// Remove deletes data from the cache for the given key.
func Remove(key string) error {
	if err := os.Remove(getPath(key)); err != nil {
		return fmt.Errorf("failed to remove cache file: %w", err)
	}

	return nil
}

func getPath(key string) string {
	hash := sha256.Sum256([]byte(key))

	return filepath.Join(cacheDir, hex.EncodeToString(hash[:]))
}

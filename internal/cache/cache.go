// Package cache provides a simple file-based cache for API responses.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"codeberg.org/upPollo/parsec/internal/config"
)

var (
	cacheDir string
	metaDir  string
	subDir   string
	fontDir  string
	mu       sync.Mutex

	errCacheBypassed = errors.New("cache bypassed")
	errCacheExpired  = errors.New("cache expired")
	errFontNotFound  = errors.New("font not found in cache")
)

const (
	cacheDuration           = 6 * time.Hour
	persistentCacheDuration = 30 * 24 * time.Hour // 30 days
	subtitleCacheDuration   = 7 * 24 * time.Hour  // 7 days
)

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
	metaDir = filepath.Join(dir, "parsec", "meta")
	subDir = filepath.Join(dir, "parsec", "subtitles")
	fontDir = filepath.Join(dir, "parsec", "fonts")
}

func setDir(dir string) {
	cacheDir = dir
	metaDir = filepath.Join(dir, "meta")
	subDir = filepath.Join(dir, "subtitles")
	fontDir = filepath.Join(dir, "fonts")
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
	removeExpiredMetaFiles()
	removeExpiredSubtitleFiles()
	removeExpiredFontFiles()
	removeOldExecutable()

	// Update marker
	_ = os.WriteFile(markerPath, []byte{}, 0o644)
}

func removeOldExecutable() {
	if exe, err := os.Executable(); err == nil {
		if realPath, err := filepath.EvalSymlinks(exe); err == nil {
			exe = realPath
		}

		_ = os.Remove(exe + ".old")
	}
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

func removeExpiredMetaFiles() {
	if _, err := os.Stat(metaDir); os.IsNotExist(err) {
		return
	}

	files, err := os.ReadDir(metaDir)
	if err != nil {
		return
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		if time.Since(info.ModTime()) > persistentCacheDuration {
			_ = os.Remove(filepath.Join(metaDir, file.Name()))
		}
	}
}

func removeExpiredSubtitleFiles() {
	if _, err := os.Stat(subDir); os.IsNotExist(err) {
		return
	}

	files, err := os.ReadDir(subDir)
	if err != nil {
		return
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		if time.Since(info.ModTime()) > subtitleCacheDuration {
			_ = os.Remove(filepath.Join(subDir, file.Name()))
		}
	}
}

func removeExpiredFontFiles() {
	if _, err := os.Stat(fontDir); os.IsNotExist(err) {
		return
	}

	files, err := os.ReadDir(fontDir)
	if err != nil {
		return
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		if time.Since(info.ModTime()) > persistentCacheDuration {
			_ = os.Remove(filepath.Join(fontDir, file.Name()))
		}
	}
}

func clearCache() {
	_ = os.RemoveAll(cacheDir)
	_ = os.RemoveAll(metaDir)
	_ = os.RemoveAll(subDir)
	_ = os.RemoveAll(fontDir)
}

// Get retrieves data from the cache for the given key.
func Get(key string) ([]byte, error) {
	return GetWithDuration(key, cacheDuration)
}

// GetWithDuration retrieves data from the cache for the given key, using a custom expiration duration.
func GetWithDuration(key string, d time.Duration) ([]byte, error) {
	mu.Lock()
	defer mu.Unlock()

	if config.NoCache {
		return nil, errCacheBypassed
	}

	path := getPath(key)

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat cache file: %w", err)
	}

	if time.Since(info.ModTime()) > d {
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
	mu.Lock()
	defer mu.Unlock()

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
	mu.Lock()
	defer mu.Unlock()

	if err := os.Remove(getPath(key)); err != nil {
		return fmt.Errorf("failed to remove cache file: %w", err)
	}

	return nil
}

func getPath(key string) string {
	hash := sha256.Sum256([]byte(key))

	return filepath.Join(cacheDir, hex.EncodeToString(hash[:]))
}

// GetPersistent retrieves data from the persistent cache for the given key.
func GetPersistent(key string) ([]byte, error) {
	mu.Lock()
	defer mu.Unlock()

	if config.NoCache {
		return nil, errCacheBypassed
	}

	path := getMetaPath(key)

	_, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat cache file: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	now := time.Now()
	_ = os.Chtimes(path, now, now)

	return data, nil
}

// SetPersistent stores data in the persistent cache for the given key.
func SetPersistent(key string, data []byte) error {
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	if err := os.WriteFile(getMetaPath(key), data, 0o644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

func getMetaPath(key string) string {
	hash := sha256.Sum256([]byte(key))

	return filepath.Join(metaDir, hex.EncodeToString(hash[:]))
}

// MovePersistent moves a cache entry from oldKey to newKey in the persistent cache.
func MovePersistent(oldKey, newKey string) error {
	mu.Lock()
	defer mu.Unlock()

	oldPath := getMetaPath(oldKey)
	newPath := getMetaPath(newKey)

	if _, err := os.Stat(oldPath); err != nil {
		return fmt.Errorf("old cache file not found: %w", err)
	}

	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("failed to move cache file: %w", err)
	}

	return nil
}

// GetSubtitle retrieves subtitle data from the cache for the given key.
func GetSubtitle(key string) ([]byte, error) {
	mu.Lock()
	defer mu.Unlock()

	if config.NoCache {
		return nil, errCacheBypassed
	}

	path := getSubPath(key)

	_, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat cache file: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	now := time.Now()
	_ = os.Chtimes(path, now, now)

	return data, nil
}

// SetSubtitle stores subtitle data in the cache for the given key.
func SetSubtitle(key string, data []byte) error {
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(subDir, 0o755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	if err := os.WriteFile(getSubPath(key), data, 0o644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

func getSubPath(key string) string {
	hash := sha256.Sum256([]byte(key))

	return filepath.Join(subDir, hex.EncodeToString(hash[:]))
}

// GetFontPath retrieves the absolute path to the cached font file for the given key, if it exists.
func GetFontPath(key string) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	if config.NoCache {
		return "", errCacheBypassed
	}

	hash := sha256.Sum256([]byte(key))
	prefix := filepath.Join(fontDir, hex.EncodeToString(hash[:]))

	// Match the exact hash + any extension
	matches, err := filepath.Glob(prefix + ".*")
	if err != nil || len(matches) == 0 {
		return "", errFontNotFound
	}

	path := matches[0]

	now := time.Now()
	_ = os.Chtimes(path, now, now)

	return path, nil
}

// SetFont stores a font file in the cache for the given key and extension, returning the absolute path.
func SetFont(key, ext string, data []byte) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(fontDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create font cache directory: %w", err)
	}

	hash := sha256.Sum256([]byte(key))
	path := filepath.Join(fontDir, hex.EncodeToString(hash[:])+ext)

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("failed to write font cache file: %w", err)
	}

	return path, nil
}

package cache

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"codeberg.org/upPollo/parsec/internal/config"
)

var cacheDir string

const cacheDuration = 6 * time.Hour

func init() {
	ResetDir()
	Cleanup()
}

func ResetDir() {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}

	cacheDir = filepath.Join(dir, "parsec", "api")
}

func SetDir(dir string) {
	cacheDir = dir
}

func Cleanup() {
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

func Clear() {
	_ = os.RemoveAll(cacheDir)
}

func Get(key string) ([]byte, error) {
	if config.NoCache {
		return nil, fmt.Errorf("cache bypassed")
	}

	path := getPath(key)

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if time.Since(info.ModTime()) > cacheDuration {
		_ = os.Remove(path)
		return nil, fmt.Errorf("cache expired")
	}

	return os.ReadFile(path)
}

func Set(key string, data []byte) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}

	return os.WriteFile(getPath(key), data, 0o644)
}

func Remove(key string) error {
	return os.Remove(getPath(key))
}

func getPath(key string) string {
	hash := sha256.Sum256([]byte(key))
	return filepath.Join(cacheDir, fmt.Sprintf("%x", hash))
}

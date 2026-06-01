package cache

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"codeberg.org/n0ne/parsec/internal/config"
)

var cacheDir string

func init() {
	ResetDir()
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

	// Cache for 24 hours
	if time.Since(info.ModTime()) > 24*time.Hour {
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

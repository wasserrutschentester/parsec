package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"codeberg.org/upPollo/parsec/internal/cache"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/ui"
	"golang.org/x/mod/semver"
)

var (
	// owner is the Codeberg user/org
	owner = "upPollo"
	// repo is the repository name
	repo = "parsec"
	// baseURL is the Codeberg API base URL
	baseURL = "https://codeberg.org/api/v1"

	httpClient = &http.Client{Timeout: 30 * time.Second}
)

// CheckForUpdateBackground checks if a new version is available in the background
func CheckForUpdateBackground(currentVersion string) {
	if config.GetDisableUpdateCheck() {
		return
	}

	const cacheKey = "latest_release_tag"

	cachedData, err := cache.Get(cacheKey)
	if err == nil {
		latestTag := string(cachedData)
		if IsNewer(latestTag, currentVersion) {
			ui.PrintWarning(fmt.Sprintf("A new version of parsec is available: %s (Current: %s).", latestTag, currentVersion))
		}
	} else {
		// Cache miss or expired, fetch in background for next time
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if rel, err := FetchLatestRelease(ctx); err == nil {
				_ = cache.Set(cacheKey, []byte(rel.TagName))
			}
		}()
	}
}

type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func FetchLatestRelease(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", baseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("codeberg returned status %s", resp.Status)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	return &rel, nil
}

func IsNewer(latest, current string) bool {
	// semver.Compare(v, w) returns +1 if v > w
	return semver.Compare(latest, current) > 0
}

// GetMatchingAsset finds the asset for the current platform
func (r *Release) GetMatchingAsset() *Asset {
	expectedAssetName := fmt.Sprintf("%s-%s-%s", repo, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		expectedAssetName += ".exe"
	}

	for _, asset := range r.Assets {
		if asset.Name == expectedAssetName {
			return &asset
		}
	}
	return nil
}

// GetChecksumsAsset finds the checksums.txt asset
func (r *Release) GetChecksumsAsset() *Asset {
	for _, asset := range r.Assets {
		if asset.Name == "checksums.txt" {
			return &asset
		}
	}
	return nil
}

func getExecutablePath() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not get executable path: %w", err)
	}

	realPath, err := filepath.EvalSymlinks(executablePath)
	if err == nil {
		executablePath = realPath
	}
	return executablePath, nil
}

// DownloadAsset downloads the asset to a temporary .new file
func DownloadAsset(ctx context.Context, url string) (string, error) {
	executablePath, err := getExecutablePath()
	if err != nil {
		return "", err
	}

	tempFile := executablePath + ".new"
	out, err := os.Create(tempFile)
	if err != nil {
		return "", fmt.Errorf("could not create temporary file: %w", err)
	}
	defer func() { _ = out.Close() }()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not download update: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: Codeberg returned status %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", fmt.Errorf("could not write update to file: %w", err)
	}

	if err := os.Chmod(tempFile, 0o755); err != nil {
		return "", fmt.Errorf("could not set permissions on update: %w", err)
	}

	return tempFile, nil
}

// VerifyChecksum downloads the checksums file and verifies the downloaded binary
func VerifyChecksum(ctx context.Context, assetName, tempFile, checksumsURL string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", checksumsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download checksums: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download checksums: Codeberg returned status %s", resp.Status)
	}

	// Expected hash from checksums.txt
	var expectedHash string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == assetName {
			expectedHash = parts[0]
			break
		}
	}

	if expectedHash == "" {
		return fmt.Errorf("checksum for %s not found in checksums.txt", assetName)
	}

	f, err := os.Open(tempFile)
	if err != nil {
		return fmt.Errorf("could not open downloaded file for verification: %w", err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	actualHash := hex.EncodeToString(h.Sum(nil))
	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// ReplaceExecutable replaces the current executable with the new one
func ReplaceExecutable(tempFile string) error {
	executablePath, err := getExecutablePath()
	if err != nil {
		return err
	}

	// Replacement logic (works on Windows too by renaming the running binary)
	oldFile := executablePath + ".old"
	_ = os.Remove(oldFile) // Ignore error if file doesn't exist

	if err := os.Rename(executablePath, oldFile); err != nil {
		if runtime.GOOS == "windows" {
			return fmt.Errorf("could not replace running binary on Windows: %w\nPlease download the new version manually from Codeberg", err)
		}
		return fmt.Errorf("could not rename current binary: %w", err)
	}

	if err := os.Rename(tempFile, executablePath); err != nil {
		_ = os.Rename(oldFile, executablePath) // Try to restore old file on failure
		return fmt.Errorf("could not replace current binary: %w", err)
	}

	_ = os.Remove(oldFile) // Clean up old file
	return nil
}

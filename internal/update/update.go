// Package update provides functionality for self-updating the parsec binary.
package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	"codeberg.org/upPollo/parsec/internal/cache"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/ui"
)

var (
	errCodebergStatus   = errors.New("codeberg returned status")
	errChecksumNotFound = errors.New("checksum not found in checksums.txt")
	errChecksumMismatch = errors.New("checksum mismatch")

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
	if !config.GetCheckUpdates() {
		return
	}

	const cacheKey = "latest_release_tag"

	cacheDur := 6 * time.Hour
	if config.GetCheckPrereleaseUpdates() {
		cacheDur = 1 * time.Hour
	}

	cachedData, err := cache.GetWithDuration(cacheKey, cacheDur)
	if err != nil {
		// Cache miss or expired, fetch in background for next time
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if rel, err := FetchLatestRelease(ctx, config.GetCheckPrereleaseUpdates()); err == nil {
				_ = cache.Set(cacheKey, []byte(rel.TagName))
			}
		}()

		return
	}

	latestTag := string(cachedData)
	ui.PrintDebug(fmt.Sprintf("cached latest tag: %s, current version: %s", latestTag, currentVersion))

	if !IsNewer(latestTag, currentVersion) {
		return
	}

	if !config.GetAutoUpdate() {
		ui.PrintWarning(fmt.Sprintf("A new version of parsec is available: %s (Current: %s).", latestTag, currentVersion))

		return
	}

	ui.PrintWarning(fmt.Sprintf("A new version of parsec is available (%s). It is downloading in the background...", latestTag))

	exe, err := os.Executable()
	if err == nil {
		cmd := exec.CommandContext(context.Background(), exe, "update", "--silent")
		setSysProcAttr(cmd)
		_ = cmd.Start() // Start in background and detach
	}
}

// Release represents a GitHub-like release from Codeberg.
type Release struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	Body    string  `json:"body"`
	Assets  []Asset `json:"assets"`
}

// Asset represents a single file asset in a release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

var errNotFound = errors.New("release not found")

// FetchLatestRelease retrieves the latest release information from the Codeberg API.
func FetchLatestRelease(ctx context.Context, checkPrerelease bool) (*Release, error) {
	stableURL := fmt.Sprintf("%s/repos/%s/%s/releases/latest", baseURL, owner, repo)

	stableRel, err := fetchRelease(ctx, stableURL)
	if err != nil && !errors.Is(err, errNotFound) {
		return nil, err
	}

	if !checkPrerelease {
		if stableRel == nil {
			return nil, errNotFound
		}

		return stableRel, nil
	}

	nightlyRel, err := fetchNightlyRelease(ctx)
	if err != nil {
		return nil, err
	}

	return pickBestRelease(stableRel, nightlyRel)
}

func fetchNightlyRelease(ctx context.Context) (*Release, error) {
	nightlyURL := fmt.Sprintf("%s/repos/%s/%s/releases/tags/nightly", baseURL, owner, repo)

	nightlyRel, err := fetchRelease(ctx, nightlyURL)
	if err != nil && !errors.Is(err, errNotFound) {
		return nil, err
	}

	if nightlyRel != nil {
		extractNightlyVersion(nightlyRel)
	}

	return nightlyRel, nil
}

func pickBestRelease(stableRel, nightlyRel *Release) (*Release, error) {
	if stableRel == nil && nightlyRel == nil {
		return nil, errNotFound
	}

	if stableRel == nil {
		return nightlyRel, nil
	}

	if nightlyRel == nil {
		return stableRel, nil
	}

	if IsNewer(nightlyRel.TagName, stableRel.TagName) {
		return nightlyRel, nil
	}

	return stableRel, nil
}

func extractNightlyVersion(nightlyRel *Release) {
	// Extract the true semantic version from the git-cliff changelog body
	// It outputs: ## [v0.4.1-dev.12+4508cc6] - 2026-07-01
	matches := regexp.MustCompile(`(?m)^## \[(v[0-9]+\.[0-9]+\.[0-9]+.*?)\]`).FindStringSubmatch(nightlyRel.Body)
	if len(matches) > 1 {
		nightlyRel.TagName = matches[1]
	} else if nightlyRel.Name != "" && nightlyRel.Name != "nightly" {
		nightlyRel.TagName = nightlyRel.Name
	}
}

func fetchRelease(ctx context.Context, url string) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w %s", errCodebergStatus, resp.Status)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	return &rel, nil
}

// IsNewer compares two semver tags and returns true if latest is newer than current.
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not download update: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: Codeberg returned status %s", errCodebergStatus, resp.Status)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download checksums: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download checksums: %w (%s)", errCodebergStatus, resp.Status)
	}

	expectedHash := getExpectedHash(resp.Body, assetName)
	if expectedHash == "" {
		return fmt.Errorf("%w: for %s", errChecksumNotFound, assetName)
	}

	actualHash, err := calculateSHA256(tempFile)
	if err != nil {
		return err
	}

	if actualHash != expectedHash {
		return fmt.Errorf("%w: expected %s, got %s", errChecksumMismatch, expectedHash, actualHash)
	}

	return nil
}

func getExpectedHash(body io.Reader, assetName string) string {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()

		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == assetName {
			return parts[0]
		}
	}

	return ""
}

func calculateSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("could not open downloaded file for verification: %w", err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
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

	if err := renameWithRetry(executablePath, oldFile); err != nil {
		if runtime.GOOS == "windows" {
			return fmt.Errorf("could not replace running binary on Windows: %w\nPlease download the new version manually from Codeberg", err)
		}

		return fmt.Errorf("could not rename current binary: %w", err)
	}

	if err := renameWithRetry(tempFile, executablePath); err != nil {
		_ = renameWithRetry(oldFile, executablePath) // Try to restore old file on failure

		return fmt.Errorf("could not replace current binary: %w", err)
	}

	_ = os.Remove(oldFile) // Clean up old file

	return nil
}

// renameWithRetry attempts to rename a file, retrying up to 5 times if it fails.
// This is necessary on Windows where AntiVirus software might temporarily lock a newly downloaded executable.
func renameWithRetry(oldpath, newpath string) error {
	var err error
	for range 5 {
		err = os.Rename(oldpath, newpath)
		if err == nil {
			return nil
		}

		time.Sleep(200 * time.Millisecond)
	}

	if err != nil {
		return fmt.Errorf("rename failed: %w", err)
	}

	return nil
}

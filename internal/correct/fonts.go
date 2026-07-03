package correct

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"codeberg.org/upPollo/parsec/internal/cache"
	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/ui"
)

const (
	fontSourceSystem        = "system"
	fontSourceGoogleGitHub  = "google-fonts-github"
	fontSourceGoogleAPI     = "google-fonts-api"
	googleFontsGitHubAPI    = "https://api.github.com/repos/google/fonts/contents"
	googleFontsDeveloperAPI = "https://www.googleapis.com/webfonts/v1/webfonts"
	maxFontDownloadSize     = 128 << 20
)

var (
	errFontTooLarge = errors.New("font exceeds maximum download size")
	errHTTPStatus   = errors.New("unexpected HTTP status")
	googleFontDirRe = regexp.MustCompile(`[^a-z0-9]+`)
)

// MissingFontAttachmentPlan describes the missing subtitle fonts that can be
// embedded and the ones no configured resolver could locate.
type MissingFontAttachmentPlan struct {
	Attachments []MissingFontAttachment
	Unresolved  []string
}

// MissingFontAttachment is one font attachment to add to a Matroska file.
type MissingFontAttachment struct {
	FontName       string
	Path           string
	AttachmentName string
	MIMEType       string
	Source         string
	InternalNames  []string
}

// ResolvedFont describes a font file found by a resolver.
type ResolvedFont struct {
	Path          string
	Source        string
	InternalNames []string
}

type fontResolver interface {
	Resolve(fontName string, allowDownload bool) (ResolvedFont, bool)
}

type defaultFontResolver struct {
	client *http.Client
}

type githubContent struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	DownloadURL string `json:"download_url"`
}

type googleFontsAPIResponse struct {
	Items []googleFontFamily `json:"items"`
}

type googleFontFamily struct {
	Family string            `json:"family"`
	Files  map[string]string `json:"files"`
}

func newDefaultFontResolver() defaultFontResolver {
	return defaultFontResolver{client: &http.Client{Timeout: 30 * time.Second}}
}

// ComputeMissingFontAttachments returns the missing ASS/SSA subtitle fonts
// that can be resolved and attached. Downloads are skipped when allowDownload
// is false, which lets dry-run previews avoid mutating the font cache.
// attachmentFonts is the caller's already-computed checks.GetAttachmentFonts
// result (see ComputeUnusedFontAttachments for why this isn't recomputed here).
func ComputeMissingFontAttachments(filePath string, ebml *matroska.EbmlMetadata, attachmentFonts []matroska.AttachmentFontInfo, allowDownload bool) MissingFontAttachmentPlan {
	missing := checks.ComputeMissingFonts(filePath, ebml.Tracks, attachmentFonts)

	return computeMissingFontAttachmentPlan(missing, ebml.Attachments, newDefaultFontResolver(), allowDownload)
}

func computeMissingFontAttachmentPlan(missing []string, existing []matroska.EbmlAttachment, resolver fontResolver, allowDownload bool) MissingFontAttachmentPlan {
	plan := MissingFontAttachmentPlan{}
	usedNames := existingAttachmentNames(existing)

	for _, fontName := range missing {
		resolved, ok := resolver.Resolve(fontName, allowDownload)
		if !ok {
			plan.Unresolved = append(plan.Unresolved, fontName)

			continue
		}

		attachmentName := uniqueAttachmentName(attachmentNameForFont(fontName, resolved.Path), usedNames)
		plan.Attachments = append(plan.Attachments, MissingFontAttachment{
			FontName:       fontName,
			Path:           resolved.Path,
			AttachmentName: attachmentName,
			MIMEType:       fontMIMEType(resolved.Path),
			Source:         resolved.Source,
			InternalNames:  resolved.InternalNames,
		})
	}

	return plan
}

func existingAttachmentNames(attachments []matroska.EbmlAttachment) map[string]bool {
	used := make(map[string]bool, len(attachments))
	for _, att := range attachments {
		used[strings.ToLower(att.FileName)] = true
	}

	return used
}

func attachmentNameForFont(fontName, path string) string {
	base := filepath.Base(path)
	if filepath.Ext(base) == "" {
		base = fontName + ".ttf"
	}

	return fontRenameTarget(base, fontName)
}

func uniqueAttachmentName(name string, used map[string]bool) string {
	candidate := name
	for n := 2; used[strings.ToLower(candidate)]; n++ {
		candidate = suffixFontName(name, n)
	}

	used[strings.ToLower(candidate)] = true

	return candidate
}

func fontMIMEType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".otf":
		return "font/otf"
	case ".ttc", ".otc":
		return "font/collection"
	case ".ttf":
		return "font/ttf"
	default:
		return "font/sfnt"
	}
}

func (r defaultFontResolver) Resolve(fontName string, allowDownload bool) (ResolvedFont, bool) {
	if resolved, ok := resolveSystemFont(fontName); ok {
		return resolved, true
	}

	if !allowDownload {
		return ResolvedFont{}, false
	}

	if resolved, ok := r.resolveGoogleFontsGitHub(fontName); ok {
		return resolved, true
	}

	if key := config.GetGoogleFontsAPIKey(); key != "" {
		if resolved, ok := r.resolveGoogleFontsAPI(fontName, key); ok {
			return resolved, true
		}
	}

	return ResolvedFont{}, false
}

func resolveSystemFont(fontName string) (ResolvedFont, bool) {
	for _, path := range fontConfigMatches(fontName) {
		if names, ok := fontFileMatches(path, fontName); ok {
			return ResolvedFont{Path: path, Source: fontSourceSystem, InternalNames: names}, true
		}
	}

	return ResolvedFont{}, false
}

func fontConfigMatches(fontName string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "fc-match", "-a", "--format", "%{file}\n", fontName)

	output, err := cmd.Output()
	if err != nil {
		ui.PrintDebug(fmt.Sprintf("fc-match failed for %s: %v", fontName, err))

		return nil
	}

	seen := make(map[string]bool)
	paths := make([]string, 0)

	for line := range strings.SplitSeq(string(output), "\n") {
		path := strings.TrimSpace(line)
		if path == "" || seen[path] {
			continue
		}

		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			seen[path] = true
			paths = append(paths, path)
		}
	}

	return paths
}

func fontFileMatches(path, fontName string) ([]string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		ui.PrintDebug(fmt.Sprintf("failed to read font %s: %v", ui.AnonymizePath(path), err))

		return nil, false
	}

	names, err := matroska.GetFontNames(data)
	if err != nil {
		ui.PrintDebug(fmt.Sprintf("failed to parse font %s: %v", ui.AnonymizePath(path), err))

		return nil, false
	}

	return names, checks.FontNameMatches(fontName, names)
}

func (r defaultFontResolver) resolveGoogleFontsGitHub(fontName string) (ResolvedFont, bool) {
	dir := googleFontDirName(fontName)
	if dir == "" {
		return ResolvedFont{}, false
	}

	for _, licenseDir := range []string{"ofl", "apache", "ufl"} {
		var contents []githubContent

		apiURL := fmt.Sprintf("%s/%s/%s?ref=main", googleFontsGitHubAPI, licenseDir, dir)
		if err := r.getJSON("font-github:"+apiURL, apiURL, &contents); err != nil {
			ui.PrintDebug(fmt.Sprintf("Google Fonts GitHub lookup failed for %s: %v", fontName, err))

			continue
		}

		for _, candidate := range googleFontCandidates(contents) {
			if resolved, ok := r.downloadAndMatchFont(candidate.DownloadURL, candidate.Name, fontName, fontSourceGoogleGitHub); ok {
				return resolved, true
			}
		}
	}

	return ResolvedFont{}, false
}

func googleFontDirName(fontName string) string {
	return googleFontDirRe.ReplaceAllString(strings.ToLower(fontName), "")
}

func googleFontCandidates(contents []githubContent) []githubContent {
	candidates := make([]githubContent, 0, len(contents))
	for _, item := range contents {
		if item.Type != "file" || item.DownloadURL == "" || !isFontFile(item.Name) {
			continue
		}

		candidates = append(candidates, item)
	}

	slices.SortFunc(candidates, func(a, b githubContent) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return candidates
}

func isFontFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".ttf", ".otf", ".ttc", ".otc":
		return true
	default:
		return false
	}
}

func (r defaultFontResolver) resolveGoogleFontsAPI(fontName, key string) (ResolvedFont, bool) {
	apiURL, err := googleFontsAPIURL(fontName, key)
	if err != nil {
		return ResolvedFont{}, false
	}

	var response googleFontsAPIResponse
	if err := r.getJSON("font-google-api:"+fontName, apiURL, &response); err != nil {
		ui.PrintDebug(fmt.Sprintf("Google Fonts Developer API lookup failed for %s: %v", fontName, err))

		return ResolvedFont{}, false
	}

	for _, family := range response.Items {
		if !checks.FontNameMatches(fontName, []string{family.Family}) {
			continue
		}

		variants := mapsKeys(family.Files)
		slices.Sort(variants)

		for _, variant := range variants {
			fontURL := family.Files[variant]
			if rest, ok := strings.CutPrefix(fontURL, "http://"); ok {
				fontURL = "https://" + rest
			}

			if resolved, ok := r.downloadAndMatchFont(fontURL, filepath.Base(fontURL), fontName, fontSourceGoogleAPI); ok {
				return resolved, true
			}
		}
	}

	return ResolvedFont{}, false
}

func googleFontsAPIURL(fontName, key string) (string, error) {
	u, err := url.Parse(googleFontsDeveloperAPI)
	if err != nil {
		return "", fmt.Errorf("parse Google Fonts Developer API URL: %w", err)
	}

	q := u.Query()
	q.Set("family", fontName)
	q.Set("key", key)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func mapsKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	return keys
}

// getJSON fetches requestURL as JSON into target, serving from the shared
// on-disk API cache (internal/cache, 6h TTL) when possible. cacheKey is kept
// separate from requestURL so callers can key on stable inputs (e.g. font
// name) rather than a URL that embeds a rotating API key, which would
// otherwise orphan the cache entry every time the key changes. This avoids
// hitting the Google Fonts GitHub API's unauthenticated 60/hour rate limit
// on every font lookup in a batch.
func (r defaultFontResolver) getJSON(cacheKey, requestURL string, target any) error {
	if cached, err := cache.Get(cacheKey); err == nil {
		if err := json.Unmarshal(cached, target); err == nil {
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return fmt.Errorf("create font metadata request: %w", err)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch font metadata: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s", errHTTPStatus, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read font metadata response: %w", err)
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode font metadata response: %w", err)
	}

	_ = cache.Set(cacheKey, body)

	return nil
}

func (r defaultFontResolver) downloadAndMatchFont(fontURL, fileName, fontName, source string) (ResolvedFont, bool) {
	path := cachedFontPath(fontURL, fileName)

	if _, err := os.Stat(path); err == nil {
		if names, ok := fontFileMatches(path, fontName); ok {
			return ResolvedFont{Path: path, Source: source, InternalNames: names}, true
		}
	}

	data, err := r.downloadFont(fontURL)
	if err != nil {
		ui.PrintDebug(fmt.Sprintf("failed to download font %s from %s: %v", fontName, fontURL, err))

		return ResolvedFont{}, false
	}

	names, err := matroska.GetFontNames(data)
	if err != nil || !checks.FontNameMatches(fontName, names) {
		return ResolvedFont{}, false
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		ui.PrintDebug(fmt.Sprintf("failed to create font cache: %v", err))

		return ResolvedFont{}, false
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		ui.PrintDebug(fmt.Sprintf("failed to write cached font %s: %v", ui.AnonymizePath(path), err))

		return ResolvedFont{}, false
	}

	return ResolvedFont{Path: path, Source: source, InternalNames: names}, true
}

func (r defaultFontResolver) downloadFont(fontURL string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fontURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create font download request: %w", err)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download font: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s", errHTTPStatus, resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFontDownloadSize+1))
	if err != nil {
		return nil, fmt.Errorf("read font download: %w", err)
	}

	if len(data) > maxFontDownloadSize {
		return nil, fmt.Errorf("%w: %d bytes", errFontTooLarge, maxFontDownloadSize)
	}

	return data, nil
}

func cachedFontPath(fontURL, fileName string) string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	if ext == "" {
		if u, err := url.Parse(fontURL); err == nil {
			ext = strings.ToLower(filepath.Ext(u.Path))
		}
	}

	if ext == "" {
		ext = ".ttf"
	}

	hash := sha256.Sum256([]byte(fontURL))

	return filepath.Join(dir, "parsec", "fonts", hex.EncodeToString(hash[:])+ext)
}

// attachMissingFonts locates missing ASS/SSA subtitle fonts and embeds the
// matching font files as Matroska attachments.
func attachMissingFonts(filePath string, ebml *matroska.EbmlMetadata, attachmentFonts []matroska.AttachmentFontInfo, opts Options) error {
	plan := ComputeMissingFontAttachments(filePath, ebml, attachmentFonts, false)
	if len(plan.Attachments) == 0 && len(plan.Unresolved) == 0 {
		return nil
	}

	printMissingFontPlan(plan, opts.DryRun, len(plan.Unresolved) > 0)

	if len(plan.Attachments) == 0 {
		if opts.DryRun {
			return nil
		}
	}

	if !confirmApplyWithPolicy(opts, "Attach these missing subtitle fonts?", "Skipping missing font attachments...", false) {
		return nil
	}

	if len(plan.Unresolved) > 0 {
		plan = ComputeMissingFontAttachments(filePath, ebml, attachmentFonts, true)
		printMissingFontPlan(plan, false, false)
	}

	if len(plan.Attachments) == 0 {
		return nil
	}

	if err := matroska.AddAttachments(filePath, fontAttachmentAdds(plan)); err != nil {
		ui.PrintError(fmt.Sprintf("Error attaching fonts for %s: %v", ui.AnonymizePath(filePath), err))

		return errTrackFix
	}

	ui.PrintSuccess("Missing subtitle fonts attached.")

	return nil
}

func printMissingFontPlan(plan MissingFontAttachmentPlan, dryRun, downloadsDeferred bool) {
	ui.Println(ui.ReportSection("Missing Subtitle Fonts"))

	if len(plan.Attachments) > 0 {
		ui.Println("  Resolved:")
	}

	for _, att := range plan.Attachments {
		ui.Println(fmt.Sprintf("    %s -> %s", quoteOrNone(att.FontName), quoteOrNone(att.AttachmentName)))
		ui.Println("      source: " + att.Source)
		ui.Println("      file:   " + ui.AnonymizePath(att.Path))
	}

	if len(plan.Unresolved) > 0 {
		ui.Println("  Unresolved:")
	}

	for _, fontName := range plan.Unresolved {
		ui.Println("    " + ui.Warning.Render(quoteOrNone(fontName)))
	}

	if dryRun && len(plan.Unresolved) > 0 {
		ui.Println(ui.Muted.Render("Remote font downloads are skipped during dry-run."))
	} else if downloadsDeferred && len(plan.Unresolved) > 0 {
		ui.Println(ui.Muted.Render("Remote font downloads are deferred until after confirmation."))
	}
}

func fontAttachmentAdds(plan MissingFontAttachmentPlan) []matroska.AttachmentAdd {
	attachments := make([]matroska.AttachmentAdd, 0, len(plan.Attachments))
	for _, att := range plan.Attachments {
		attachments = append(attachments, matroska.AttachmentAdd{
			Path:     att.Path,
			Name:     att.AttachmentName,
			MIMEType: att.MIMEType,
		})
	}

	return attachments
}

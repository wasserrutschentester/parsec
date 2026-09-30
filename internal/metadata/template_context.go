package metadata

import (
	"bytes"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/templateutil"
	"codeberg.org/upPollo/parsec/internal/ui"
)

// TemplateContext wraps Metadata via embedding and provides convenience fields for
// common release naming patterns alongside raw context objects for deep inspection.
type TemplateContext struct {
	Metadata

	// Precomputed convenience fields (used in templates & release naming)
	YearTag       string `json:"YearTag"`       // Formatted release year (e.g. "2024") or "" if unknown/zero
	SeasonID      string `json:"SeasonID"`      // "S01", "S00" for specials, or "" for movies
	EpisodeID     string `json:"EpisodeID"`     // "E01", "E01-E05", or "" for movies
	SeasonEpisode string `json:"SeasonEpisode"` // Combined "S01E01", "S01" for packs, or "" for movies
	EpisodeTitle  string `json:"EpisodeTitle"`  // Joined episode titles ("Pilot") or ""
	AudioSpec     string `json:"AudioSpec"`     // Merged audio spec: AudioCodec + AudioChannels, plus AudioExtra if present ("DDP5.1.Atmos", "DDP5.1")
	LanguageName  string `json:"LanguageName"`  // Full uppercase language ("GERMAN", "ENGLISH")
	RepackTag     string `json:"RepackTag"`     // Formatted repack token ("REPACK", "REPACK2", or "")
	VersionTag    string `json:"VersionTag"`    // Formatted release version token ("v2", "v3", or "")
	IsPack        bool   `json:"IsPack"`        // True if season pack (matches nfo.Context.IsPack)

	// Raw context objects (for advanced querying with where/pluck and --dump-context-raw)
	RawMediaInfo any `json:"RawMediaInfo,omitempty"`
	RawMDB       any `json:"RawMDB,omitempty"`
}

// ToTemplateContext converts Metadata to TemplateContext without raw context objects.
func (meta *Metadata) ToTemplateContext() TemplateContext {
	return meta.ToTemplateContextWithRaw(nil, nil)
}

// ToTemplateContextWithRaw converts Metadata to TemplateContext with explicit raw context objects.
//
//nolint:cyclop,funlen // context initialization maps domain fields into flat convenience properties
func (meta *Metadata) ToTemplateContextWithRaw(mi, mdbRes any) TemplateContext {
	ctx := TemplateContext{
		Metadata:     *meta,
		RawMediaInfo: mi,
		RawMDB:       mdbRes,
	}

	// Sanitize OriginalTitle: if identical to Title, treat as empty
	if ctx.OriginalTitle == ctx.Title {
		ctx.OriginalTitle = ""
	}

	// Formatted Year Tag (e.g. "2024" or "" if unknown/zero)
	if meta.Year > 0 {
		ctx.YearTag = strconv.Itoa(meta.Year)
	}

	// Season Pack indicator (matches nfo.Context.IsPack)
	if (meta.Season > 0 || meta.IsTV) && len(meta.Episodes) == 0 {
		ctx.IsPack = true
	}

	// Season ID
	if meta.Season > 0 || meta.IsTV {
		ctx.SeasonID = fmt.Sprintf("S%02d", meta.Season)
	}

	// Episode ID
	ctx.EpisodeID = computeEpisodeID(meta.Episodes)

	// Combined Season + Episode (S01E01, S01, or "")
	ctx.SeasonEpisode = ctx.SeasonID + ctx.EpisodeID

	// Episode Title
	if len(meta.EpisodeTitles) > 0 {
		ctx.EpisodeTitle = strings.Join(meta.EpisodeTitles, " ")
	}

	// Audio Specification: merges AudioCodec + AudioChannels, appending AudioExtra if present (e.g. "DDP5.1.Atmos" or "DDP5.1")
	if meta.AudioCodec != "" || meta.AudioChannels != "" {
		spec := meta.AudioCodec + meta.AudioChannels
		if meta.AudioExtra != "" {
			spec += "." + meta.AudioExtra
		}

		ctx.AudioSpec = spec
	} else if meta.AudioExtra != "" {
		ctx.AudioSpec = meta.AudioExtra
	}

	// Full Language Name (e.g. "GERMAN")
	if meta.LanguageISO != "" {
		ctx.LanguageName = LanguageName(meta.LanguageISO)
	}

	// Repack Tag
	if meta.RepackLevel > 1 {
		ctx.RepackTag = fmt.Sprintf("REPACK%d", meta.RepackLevel)
	} else if meta.IsRepack || meta.RepackLevel == 1 {
		ctx.RepackTag = "REPACK"
	}

	// Version Tag (e.g. "v2", "v3", or "")
	if meta.ReleaseVersion > 1 {
		ctx.VersionTag = fmt.Sprintf("v%d", meta.ReleaseVersion)
	}

	// Audio Description fallback if accessibility tag is empty
	if meta.HasAudioDesc && ctx.Accessibility == "" {
		ctx.Accessibility = "with.Audio.Description"
	}

	// Clean CRC32 (uppercase, no brackets)
	if ctx.CRC32 != "" {
		ctx.CRC32 = strings.ToUpper(strings.Trim(ctx.CRC32, "[]"))
	}

	// Auto-resolve VideoCodec based on Codec Styling Matrix
	style := DetermineCodecStyle(meta)
	ctx.VideoCodec = FormatVideoCodec(meta.VideoCodec, style)

	return ctx
}

func computeEpisodeID(episodes []int) string {
	if len(episodes) == 0 {
		return ""
	}

	first, last := slices.Min(episodes), slices.Max(episodes)
	if first == last {
		return fmt.Sprintf("E%02d", first)
	}

	return fmt.Sprintf("E%02d-E%02d", first, last)
}

// GetReleaseName returns the full release name generated from this template context.
func (ctx *TemplateContext) GetReleaseName() string {
	template := config.GetTemplate()
	ui.PrintDebug("using template: " + template)

	return ctx.Render(template)
}

// GetSeasonPackName returns a folder name for a season pack, omitting episode-specific details.
func (ctx *TemplateContext) GetSeasonPackName() string {
	ctxCopy := *ctx
	ctxCopy.Episodes = nil
	ctxCopy.EpisodeTitles = nil
	ctxCopy.Date = ""
	ctxCopy.EpisodeID = ""
	ctxCopy.EpisodeTitle = ""
	ctxCopy.SeasonEpisode = ctxCopy.SeasonID
	ctxCopy.IsPack = true

	return ctxCopy.GetReleaseName()
}

// Render renders the template with this context.
func (ctx *TemplateContext) Render(tmpl string) string {
	rawTmpl := tmpl
	if !strings.Contains(tmpl, "{{") {
		tmpl = templateutil.TranspileLegacyTemplate(tmpl)
	}

	parsed, err := template.New("release").Funcs(ReleaseTemplateFuncMap()).Parse(tmpl)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to parse template %q: %v", tmpl, err))

		return ""
	}

	var buf bytes.Buffer
	if err := parsed.Execute(&buf, ctx); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to execute template: %v", err))

		return ""
	}

	finalName := cleanName(buf.String())
	sep := config.GetWordSeparator()

	if sep != " " {
		finalName = strings.ReplaceAll(finalName, " ", sep)
	}

	// Filename length safeguard
	finalName = ctx.truncateIfTooLong(finalName, rawTmpl)

	return finalName
}

func (ctx *TemplateContext) truncateIfTooLong(finalName, template string) string {
	if len(finalName) <= 245 {
		return finalName
	}

	ui.PrintWarning(fmt.Sprintf("Generated filename exceeds 245 bytes (%d bytes). Attempting to truncate.", len(finalName)))

	if len(ctx.EpisodeTitles) == 0 {
		ui.PrintError("Cannot truncate: no episode title to remove. This might cause filesystem errors.")

		return finalName
	}

	ctxCopy := *ctx
	ctxCopy.EpisodeTitles = nil
	ctxCopy.EpisodeTitle = ""

	// Recursively render without episode title
	truncatedName := ctxCopy.Render(template)

	if len(truncatedName) <= 245 {
		ui.PrintInfo("Successfully truncated by removing the episode title.")
	} else {
		ui.PrintError(fmt.Sprintf("Even without the episode title, the filename is still too long (%d bytes). This might cause filesystem errors.", len(truncatedName)))
	}

	return truncatedName
}

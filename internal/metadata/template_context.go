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
	ConvenienceFields

	EpisodeTitle string `json:"EpisodeTitle"`
	IsPack       bool   `json:"IsPack"` // True if season pack (matches nfo.Context.IsPack)

	// Raw context objects (for advanced querying with where/pluck and --dump-context-raw)
	RawMediaInfo      any `json:"RawMediaInfo,omitempty"`
	RawEbmlMetadata   any `json:"RawEbmlMetadata,omitempty"`
	RawSearchResult   any `json:"RawSearchResult,omitempty"`
	RawEpisodeResults any `json:"RawEpisodeResults,omitempty"`
}

// ToTemplateContext converts Metadata to TemplateContext without raw context objects.
func (meta *Metadata) ToTemplateContext() TemplateContext {
	return meta.ToTemplateContextWithRaw(nil, nil, nil, nil)
}

// ToTemplateContextWithRaw converts Metadata to TemplateContext with explicit raw context objects.
func (meta *Metadata) ToTemplateContextWithRaw(mi, mdbRes, ebml, episodeResults any) TemplateContext {
	ctx := TemplateContext{
		Metadata:          *meta,
		ConvenienceFields: ComputeConvenienceFields(meta),
		RawMediaInfo:      mi,
		RawSearchResult:   mdbRes,
		RawEbmlMetadata:   ebml,
		RawEpisodeResults: episodeResults,
	}

	// Sanitize OriginalTitle: if identical to Title, treat as empty
	if ctx.OriginalTitle == ctx.Title {
		ctx.OriginalTitle = ""
	}

	// Season Pack indicator (matches nfo.Context.IsPack)
	if (meta.Season > 0 || meta.IsTV) && len(meta.Episodes) == 0 {
		ctx.IsPack = true
	}

	// Episode Title
	if len(meta.EpisodeTitles) > 0 {
		ctx.EpisodeTitle = strings.Join(meta.EpisodeTitles, " ")
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

// ConvenienceFields holds precomputed display-ready fields derived from Metadata.
// Used by both TemplateContext and nfo.FileContext via ComputeConvenienceFields.
type ConvenienceFields struct {
	YearTag       string
	SeasonID      string
	EpisodeID     string
	SeasonEpisode string
	AudioSpec     string
	LanguageName  string
	RepackTag     string
	VersionTag    string
}

// ComputeConvenienceFields derives the shared display-ready fields from meta.
//
//nolint:cyclop // Field assignment requires bunch of if statements
func ComputeConvenienceFields(meta *Metadata) ConvenienceFields {
	var f ConvenienceFields

	if meta.Year > 0 {
		f.YearTag = strconv.Itoa(meta.Year)
	}

	if meta.Season > 0 || meta.IsTV {
		f.SeasonID = fmt.Sprintf("S%02d", meta.Season)
	}

	f.EpisodeID = computeEpisodeID(meta.Episodes)
	f.SeasonEpisode = f.SeasonID + f.EpisodeID

	if meta.AudioCodec != "" || meta.AudioChannels != "" {
		spec := meta.AudioCodec + meta.AudioChannels
		if meta.AudioExtra != "" {
			spec += "." + meta.AudioExtra
		}

		f.AudioSpec = spec
	} else if meta.AudioExtra != "" {
		f.AudioSpec = meta.AudioExtra
	}

	if meta.LanguageISO != "" {
		f.LanguageName = LanguageName(meta.LanguageISO)
	}

	if meta.RepackLevel > 1 {
		f.RepackTag = fmt.Sprintf("REPACK%d", meta.RepackLevel)
	} else if meta.IsRepack || meta.RepackLevel == 1 {
		f.RepackTag = "REPACK"
	}

	if meta.ReleaseVersion > 1 {
		f.VersionTag = fmt.Sprintf("v%d", meta.ReleaseVersion)
	}

	return f
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

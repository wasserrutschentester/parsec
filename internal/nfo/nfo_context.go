package nfo

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/metadata/mediainfo"
)

// EpisodeContext represents a distinct episode in a multi-episode file
type EpisodeContext struct {
	Number int
	Title  string
	Plot   string
	Date   string
}

// FileContext holds everything strictly related to a single media file
type FileContext struct {
	metadata.Metadata

	ReleaseName  string
	TvdbOrder    string
	EpisodeTitle string
	Size         string
	SizeBytes    int64
	Duration     string
	DurationSec  int
	Video        Video
	Audio        []Audio
	Subtitles    []Subtitle
	Plot         string

	RawEpisodeResults []mdb.EpisodeResult    `json:"RawEpisodeResults,omitempty"`
	RawMediaInfo      *mediainfo.MediaInfo   `json:"RawMediaInfo,omitempty"`
	RawEbmlMetadata   *matroska.EbmlMetadata `json:"RawEbmlMetadata,omitempty"`

	EpisodeList []EpisodeContext
}

// Context holds template variables
type Context struct {
	FileContext

	IsPack bool
	Files  []FileContext

	ImdbURL string
	TmdbURL string
	TvdbURL string
	Notes   string
	Sources []string

	RawSearchResult *mdb.SearchResult `json:"RawSearchResult,omitempty"`

	LineWidth   int
	AppVersion  string
	ServiceName string
	Today       time.Time
}

// SetLineWidth updates the global line width for the context.
func (c *Context) SetLineWidth(width int) string {
	c.LineWidth = width

	return ""
}

// WithLineWidth returns a shallow copy of the Context with the LineWidth overridden.
func (c *Context) WithLineWidth(width int) *Context {
	clone := *c
	clone.LineWidth = width

	return &clone
}

// Flags holds boolean flags for a track
type Flags struct {
	Default          bool
	Forced           bool
	HearingImpaired  bool
	VisualImpaired   bool
	TextDescriptions bool
	Original         bool
	Commentary       bool
}

// Video holds video track metadata
type Video struct {
	Title             string
	Codec             string
	CodecID           string
	Format            string
	Bitrate           string
	BitRateMode       string
	Profile           string
	Level             string
	Settings          string
	Library           string
	LibrarySettings   string
	CRF               string
	Dimensions        string
	Width             int
	Height            int
	AspectRatio       string
	Resolution        string
	Framerate         string
	BitDepth          int
	ChromaSubsampling string
	HDRFormat         string
	ScanType          string
	Flags             Flags
	Source            string
}

// Audio holds audio metadata
type Audio struct {
	Title               string
	Language            string
	Codec               string
	Channels            string
	Bitrate             string
	BitRateMode         string
	Profile             string
	SamplingRate        string
	BitDepth            int
	DialogNormalization string
	Atmos               bool
	Flags               Flags
	Source              string
}

// Subtitle holds subtitle metadata
type Subtitle struct {
	Title        string
	Language     string
	Format       string
	ElementCount int
	Flags        Flags
	SDH          bool
	Source       string
}

func getTracksByType(info *mediainfo.MediaInfo, trackType string) []*mediainfo.Track {
	var tracks []*mediainfo.Track

	for i := range info.Media.Tracks {
		if info.Media.Tracks[i].Type == trackType {
			tracks = append(tracks, &info.Media.Tracks[i])
		}
	}

	return tracks
}

func getTrackType(info *mediainfo.MediaInfo, trackType string) *mediainfo.Track {
	idx := slices.IndexFunc(info.Media.Tracks, func(t mediainfo.Track) bool {
		return t.Type == trackType
	})
	if idx >= 0 {
		return &info.Media.Tracks[idx]
	}

	return nil
}

// FileInput holds the raw parsed structures for a single media file.
type FileInput struct {
	Path           string
	Meta           *metadata.Metadata
	Info           *mediainfo.MediaInfo
	Ebml           *matroska.EbmlMetadata
	EpisodeResults []mdb.EpisodeResult
}

func populateFileInfo(fctx *FileContext, info *mediainfo.MediaInfo, ebml *matroska.EbmlMetadata) {
	if genTrack := getTrackType(info, "General"); genTrack != nil && genTrack.Duration != nil {
		durSeconds := int(*genTrack.Duration)
		fctx.DurationSec = durSeconds
		h := durSeconds / 3600
		m := (durSeconds % 3600) / 60
		s := durSeconds % 60

		if h > 0 {
			fctx.Duration = fmt.Sprintf("%d h %d min", h, m)
		} else {
			fctx.Duration = fmt.Sprintf("%d min %d s", m, s)
		}
	}

	if vTrack := getTrackType(info, "Video"); vTrack != nil {
		populateVideoContext(fctx, vTrack, ebml)
	}

	populateAudioContext(fctx, info, ebml)
	populateTextContext(fctx, info, ebml)
}

// BuildFileContext creates a FileContext for a single file
func BuildFileContext(in FileInput) FileContext {
	fctx := FileContext{
		Metadata:          *in.Meta,
		ReleaseName:       strings.TrimSuffix(filepath.Base(in.Path), filepath.Ext(in.Path)),
		TvdbOrder:         config.GetTvdbOrder(),
		EpisodeTitle:      strings.Join(in.Meta.EpisodeTitles, " / "),
		RawEpisodeResults: in.EpisodeResults,
		RawMediaInfo:      in.Info,
		RawEbmlMetadata:   in.Ebml,
	}

	populateFromEpisodeResults(&fctx, in.EpisodeResults)

	if len(in.Meta.Episodes) > 0 {
		buildEpisodeContext(&fctx, in)
	}

	stat, err := os.Stat(in.Path)
	if err == nil {
		fctx.SizeBytes = stat.Size()
		fctx.Size = fmt.Sprintf("%.2f GiB", float64(stat.Size())/(1024*1024*1024))
	}

	if in.Info != nil {
		populateFileInfo(&fctx, in.Info, in.Ebml)
	}

	return fctx
}

func buildEpisodeContext(fctx *FileContext, in FileInput) {
	fctx.EpisodeList = make([]EpisodeContext, len(in.Meta.Episodes))
	for i, epNum := range in.Meta.Episodes {
		epCtx := EpisodeContext{
			Number: epNum,
		}
		if i < len(in.EpisodeResults) {
			epCtx.Title = in.EpisodeResults[i].Name
			epCtx.Plot = in.EpisodeResults[i].Overview
			epCtx.Date = in.EpisodeResults[i].Airdate
		} else {
			if fctx.EpisodeTitle != "" {
				epCtx.Title = fctx.EpisodeTitle
			} else {
				epCtx.Title = fmt.Sprintf("Episode %d", epNum)
			}

			epCtx.Plot = fctx.Plot
			epCtx.Date = fctx.Date
		}

		fctx.EpisodeList[i] = epCtx
	}
}

func buildSingleFileContext(ctx *Context, file FileInput) {
	ctx.IsPack = false
	fctx := BuildFileContext(file)

	if fctx.Plot == "" {
		fctx.Plot = ctx.Plot
	}

	title := ctx.Title
	year := ctx.Year

	date := ctx.Date
	if fctx.Date != "" {
		date = fctx.Date
	}

	ctx.FileContext = fctx

	ctx.Title = title
	if year > 0 {
		ctx.Year = year
	}

	ctx.Date = date
}

// BuildContext builds the NFO template variables given metadata and mediainfo outputs.
func BuildContext(releaseName string, baseMeta *metadata.Metadata, files []FileInput, notes string, searchResult *mdb.SearchResult, appVersion string) *Context {
	ctx := Context{
		FileContext: FileContext{
			Metadata:    *baseMeta,
			ReleaseName: releaseName,
			TvdbOrder:   config.GetTvdbOrder(),
		},
		Notes:           notes,
		RawSearchResult: searchResult,
		LineWidth:       72, // Default line width
		AppVersion:      appVersion,
		ServiceName:     expandServiceName(baseMeta.Service),
		Today:           time.Now(),
	}

	populateFromSearchResult(&ctx, searchResult)

	if len(files) == 1 {
		buildSingleFileContext(&ctx, files[0])
	} else if len(files) > 1 {
		buildPackFileContext(&ctx, files)
	}

	return &ctx
}

func getEbmlTrackFlags(ebmlMeta *matroska.EbmlMetadata, ebmlType string, typeOrder int, defaultFlags Flags) Flags {
	if ebmlMeta == nil {
		return defaultFlags
	}

	for _, track := range ebmlMeta.Tracks {
		if (track.Type == ebmlType || (ebmlType == "subtitles" && track.Type == "subtitle")) && track.TypeOrder == typeOrder {
			return Flags{
				Default:          track.Properties.Default,
				Forced:           track.Properties.Forced,
				HearingImpaired:  track.Properties.HearingImpaired,
				VisualImpaired:   track.Properties.VisualImpaired,
				TextDescriptions: track.Properties.TextDescriptions,
				Original:         track.Properties.OriginalLanguage,
				Commentary:       track.Properties.Commentary,
			}
		}
	}

	return defaultFlags
}

func populateFromSearchResult(ctx *Context, searchResult *mdb.SearchResult) {
	if searchResult == nil {
		populateURLsFromContext(ctx)

		return
	}

	if searchResult.Title != "" {
		ctx.Title = searchResult.Title
	}

	if searchResult.Overview != "" {
		ctx.Plot = searchResult.Overview
	}

	populateURLsFromSearchResult(ctx, searchResult)

	if searchResult.Year > 0 {
		ctx.Year = searchResult.Year
	}
}

func populateURLsFromContext(ctx *Context) {
	if ctx.ImdbID != "" {
		ctx.ImdbURL = "https://www.imdb.com/title/" + ctx.ImdbID
	}

	if ctx.TmdbID > 0 {
		if ctx.IsTV {
			ctx.TmdbURL = fmt.Sprintf("https://www.themoviedb.org/tv/%d", ctx.TmdbID)
		} else {
			ctx.TmdbURL = fmt.Sprintf("https://www.themoviedb.org/movie/%d", ctx.TmdbID)
		}
	}

	if ctx.TvdbID > 0 {
		if ctx.IsTV {
			ctx.TvdbURL = fmt.Sprintf("https://thetvdb.com/?tab=series&id=%d", ctx.TvdbID)
		} else {
			ctx.TvdbURL = fmt.Sprintf("https://thetvdb.com/?tab=movie&id=%d", ctx.TvdbID)
		}
	}
}

func populateURLsFromSearchResult(ctx *Context, searchResult *mdb.SearchResult) {
	if searchResult.ImdbID != "" {
		ctx.ImdbID = searchResult.ImdbID
		ctx.ImdbURL = "https://www.imdb.com/title/" + searchResult.ImdbID
	} else if ctx.ImdbID != "" {
		ctx.ImdbURL = "https://www.imdb.com/title/" + ctx.ImdbID
	}

	if searchResult.TmdbID > 0 {
		ctx.TmdbID = searchResult.TmdbID
		ctx.TmdbURL = fmt.Sprintf("https://www.themoviedb.org/%s/%d", searchResult.TmdbType, searchResult.TmdbID)
	}

	if searchResult.TvdbID > 0 {
		ctx.TvdbID = searchResult.TvdbID
		if searchResult.TvdbSlug != "" {
			ctx.TvdbURL = fmt.Sprintf("https://thetvdb.com/%s/%s", searchResult.TvdbType, searchResult.TvdbSlug)
		} else {
			ctx.TvdbURL = fmt.Sprintf("https://thetvdb.com/?tab=%s&id=%d", searchResult.TvdbType, searchResult.TvdbID)
		}
	}
}

func populateFromEpisodeResults(fctx *FileContext, episodeResults []mdb.EpisodeResult) {
	if len(episodeResults) == 0 {
		return
	}

	epNames := make([]string, 0, len(episodeResults))
	plots := make([]string, 0, len(episodeResults))

	for _, ep := range episodeResults {
		epNames = append(epNames, ep.Name)

		if ep.Overview != "" {
			plots = append(plots, ep.Overview)
		}

		if fctx.Date == "" { // Override date with airdate if first episode
			fctx.Date = ep.Airdate
		}
	}

	if len(epNames) > 0 {
		fctx.EpisodeTitle = strings.Join(epNames, " / ")
	}

	if len(plots) > 0 {
		fctx.Plot = strings.Join(plots, "\n\n")
	}
}

func populateVideoContext(fctx *FileContext, vTrack *mediainfo.Track, ebmlMeta *matroska.EbmlMetadata) {
	fctx.Video.Title = vTrack.Title
	fctx.Video.Codec = metadata.VideoCodecName(vTrack.Format, vTrack.FormatVersion, vTrack.CodecIDHint)
	fctx.Video.CodecID = vTrack.CodecID
	fctx.Video.Format = vTrack.Format
	fctx.Video.Profile = vTrack.FormatProfile
	fctx.Video.Level = vTrack.FormatLevel
	fctx.Video.BitRateMode = vTrack.BitRateMode
	fctx.Video.BitDepth = vTrack.BitDepth
	fctx.Video.ChromaSubsampling = vTrack.ChromaSubsampling
	fctx.Video.ScanType = vTrack.ScanType

	if vTrack.HDRFormat != "" {
		fctx.Video.HDRFormat = vTrack.HDRFormat
	} else if vTrack.HDRFormatCompatibility != "" {
		fctx.Video.HDRFormat = vTrack.HDRFormatCompatibility
	}

	baseFlags := Flags{
		Default: bool(vTrack.Default),
		Forced:  bool(vTrack.Forced),
	}
	fctx.Video.Flags = getEbmlTrackFlags(ebmlMeta, "video", 1, baseFlags)

	var settings []string
	if cabac := vTrack.FormatSettingsCABAC; cabac == "Yes" {
		settings = append(settings, "CABAC")
	}

	if ref := vTrack.FormatSettingsRefFrames; ref != "" {
		settings = append(settings, ref+" Ref Frames")
	}

	fctx.Video.Settings = strings.Join(settings, " / ")

	fctx.Video.Library = vTrack.EncodedLibrary
	fctx.Video.LibrarySettings = vTrack.EncodedLibrarySettings
	fctx.Video.CRF = extractCRF(vTrack.EncodedLibrarySettings)

	fctx.Video.Bitrate = fmt.Sprintf("%d kb/s", vTrack.BitRate/1000)
	fctx.Video.Dimensions = fmt.Sprintf("%dx%d", vTrack.Width, vTrack.Height)
	fctx.Video.Width = vTrack.Width
	fctx.Video.Height = vTrack.Height

	populateVideoAspectAndResolution(fctx, vTrack)
}

func populateVideoAspectAndResolution(fctx *FileContext, vTrack *mediainfo.Track) {
	// Calculate Aspect Ratio using GCD
	a, b := vTrack.Width, vTrack.Height
	for b != 0 {
		t := b
		b = a % b
		a = t
	}

	if a > 0 {
		fctx.Video.AspectRatio = fmt.Sprintf("%d:%d", vTrack.Width/a, vTrack.Height/a)
	} else if vTrack.DisplayAspectRatio > 0 {
		fctx.Video.AspectRatio = fmt.Sprintf("%.3f", vTrack.DisplayAspectRatio)
	}

	fctx.Video.Resolution = metadata.HeightToResolution(vTrack.Height, vTrack.ScanType, vTrack.FrameRate)
	if fctx.Video.Resolution == "" {
		fctx.Video.Resolution = fmt.Sprintf("%vx%v", vTrack.Width, vTrack.Height)
	}

	if vTrack.FrameRate > 0 {
		fctx.Video.Framerate = fmt.Sprintf("%.3f FPS", vTrack.FrameRate)
	}

	if fctx.Resolution == "" {
		fctx.Resolution = fctx.Video.Resolution
	}
}

func populateAudioContext(fctx *FileContext, info *mediainfo.MediaInfo, ebmlMeta *matroska.EbmlMetadata) {
	for i, aTrack := range getTracksByType(info, "Audio") {
		isAtmos := strings.Contains(strings.ToLower(aTrack.FormatCommercial), "atmos") ||
			strings.Contains(strings.ToLower(aTrack.FormatCommercialIfAny), "atmos")

		audio := Audio{
			Title:               aTrack.Title,
			Language:            aTrack.Language,
			Codec:               metadata.AudioCodecName(aTrack.Format, aTrack.FormatProfile, aTrack.FormatAdditionalFeatures),
			Channels:            metadata.ChanToNotation(aTrack.Channels),
			Bitrate:             fmt.Sprintf("%d kb/s", aTrack.BitRate/1000),
			BitRateMode:         aTrack.BitRateMode,
			Profile:             aTrack.FormatProfile,
			BitDepth:            aTrack.BitDepth,
			DialogNormalization: aTrack.DialogNormalization,
			Atmos:               isAtmos,
			Flags: getEbmlTrackFlags(ebmlMeta, "audio", i+1, Flags{
				Default: bool(aTrack.Default),
				Forced:  bool(aTrack.Forced),
			}),
		}
		if aTrack.SamplingRate > 0 {
			audio.SamplingRate = fmt.Sprintf("%.1f kHz", float64(aTrack.SamplingRate)/1000.0)
		}

		fctx.Audio = append(fctx.Audio, audio)
	}
}

func extractCRF(settings string) string {
	for part := range strings.SplitSeq(settings, " / ") {
		if after, ok := strings.CutPrefix(strings.TrimSpace(part), "crf="); ok {
			return " (crf" + after + ")"
		}
	}

	return ""
}

func populateTextContext(fctx *FileContext, info *mediainfo.MediaInfo, ebmlMeta *matroska.EbmlMetadata) {
	for i, tTrack := range getTracksByType(info, "Text") {
		isSDH := strings.Contains(strings.ToLower(tTrack.Title), "sdh") ||
			strings.Contains(strings.ToLower(tTrack.Title), "hearing impaired")

		elemCount := 0
		if tTrack.ElementCount != nil {
			elemCount = *tTrack.ElementCount
		}

		fctx.Subtitles = append(fctx.Subtitles, Subtitle{
			Title:        tTrack.Title,
			Language:     tTrack.Language,
			Format:       tTrack.Format,
			ElementCount: elemCount,
			Flags: getEbmlTrackFlags(ebmlMeta, "subtitles", i+1, Flags{
				Default: bool(tTrack.Default),
				Forced:  bool(tTrack.Forced),
			}),
			SDH: isSDH,
		})
	}
}

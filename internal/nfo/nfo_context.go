// Package nfo provides logic for generating and rendering NFO templates.
package nfo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/metadata/mediainfo"
)

// Context holds template variables
type Context struct {
	metadata.Metadata

	ReleaseName  string
	EpisodeTitle string
	Size         string
	SizeBytes    int64
	ImdbURL      string
	TmdbURL      string
	TvdbURL      string
	Duration     string
	DurationSec  int
	Notes        string
	Video        Video
	Audio        []Audio
	Subtitles    []Subtitle
	Plot         string
	Sources      []string

	RawSearchResult   *mdb.SearchResult      `json:"RawSearchResult,omitempty"`
	RawEpisodeResults []mdb.EpisodeResult    `json:"RawEpisodeResults,omitempty"`
	RawMediaInfo      *mediainfo.MediaInfo   `json:"RawMediaInfo,omitempty"`
	RawEbmlMetadata   *matroska.EbmlMetadata `json:"RawEbmlMetadata,omitempty"`

	LineWidth   int
	AppVersion  string
	ServiceName string
}

// SetLineWidth updates the global line width for the context.
func (c *Context) SetLineWidth(width int) string {
	c.LineWidth = width

	return ""
}

// WithLineWidth returns a shallow copy of the Context with the LineWidth overridden.
// This is incredibly useful for rendering partials with a custom width without mutating global state.
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
	Dimensions        string
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
	for i := range info.Media.Tracks {
		if info.Media.Tracks[i].Type == trackType {
			return &info.Media.Tracks[i]
		}
	}

	return nil
}

// BuildContext builds the NFO template variables given metadata and mediainfo outputs.
func BuildContext(meta *metadata.Metadata, info *mediainfo.MediaInfo, file string, notes string, searchResult *mdb.SearchResult, episodeResults []mdb.EpisodeResult, ebmlMeta *matroska.EbmlMetadata, appVersion string) *Context {
	ctx := Context{
		Metadata:          *meta,
		ReleaseName:       strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)),
		EpisodeTitle:      strings.Join(meta.EpisodeTitles, " / "),
		Notes:             notes,
		RawSearchResult:   searchResult,
		RawEpisodeResults: episodeResults,
		RawMediaInfo:      info,
		RawEbmlMetadata:   ebmlMeta,
		LineWidth:         72, // Default line width
		AppVersion:        appVersion,
		ServiceName:       expandServiceName(meta.Service),
	}

	populateFromMDB(&ctx, searchResult, episodeResults)

	stat, err := os.Stat(file)
	if err == nil {
		ctx.SizeBytes = stat.Size()
		ctx.Size = fmt.Sprintf("%.2f GiB", float64(stat.Size())/(1024*1024*1024))
	}

	if info == nil {
		return &ctx
	}

	if genTrack := getTrackType(info, "General"); genTrack != nil && genTrack.Duration != nil {
		durSeconds := int(*genTrack.Duration)
		ctx.DurationSec = durSeconds
		h := durSeconds / 3600
		m := (durSeconds % 3600) / 60
		s := durSeconds % 60

		if h > 0 {
			ctx.Duration = fmt.Sprintf("%d h %d min", h, m)
		} else {
			ctx.Duration = fmt.Sprintf("%d min %d s", m, s)
		}
	}

	if vTrack := getTrackType(info, "Video"); vTrack != nil {
		populateVideoContext(&ctx, vTrack, ebmlMeta)
	}

	populateAudioContext(&ctx, info, ebmlMeta)
	populateTextContext(&ctx, info, ebmlMeta)

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

func populateFromMDB(ctx *Context, searchResult *mdb.SearchResult, episodeResults []mdb.EpisodeResult) {
	populateFromSearchResult(ctx, searchResult)
	populateFromEpisodeResults(ctx, episodeResults)
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

func populateFromEpisodeResults(ctx *Context, episodeResults []mdb.EpisodeResult) {
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

		if ctx.Date == "" { // Override date with airdate if first episode
			ctx.Date = ep.Airdate
		}
	}

	if len(epNames) > 0 {
		ctx.EpisodeTitle = strings.Join(epNames, " / ")
	}

	if len(plots) > 0 {
		ctx.Plot = strings.Join(plots, "\n\n")
	}
}

func populateVideoContext(ctx *Context, vTrack *mediainfo.Track, ebmlMeta *matroska.EbmlMetadata) {
	ctx.Video.Title = vTrack.Title
	ctx.Video.Codec = metadata.VideoCodecName(vTrack.Format, vTrack.FormatVersion, vTrack.CodecIDHint)
	ctx.Video.CodecID = vTrack.CodecID
	ctx.Video.Format = vTrack.Format
	ctx.Video.Profile = vTrack.FormatProfile
	ctx.Video.Level = vTrack.FormatLevel
	ctx.Video.BitRateMode = vTrack.BitRateMode
	ctx.Video.BitDepth = vTrack.BitDepth
	ctx.Video.ChromaSubsampling = vTrack.ChromaSubsampling
	ctx.Video.ScanType = vTrack.ScanType

	if vTrack.HDRFormat != "" {
		ctx.Video.HDRFormat = vTrack.HDRFormat
	} else if vTrack.HDRFormatCompatibility != "" {
		ctx.Video.HDRFormat = vTrack.HDRFormatCompatibility
	}

	baseFlags := Flags{
		Default: bool(vTrack.Default),
		Forced:  bool(vTrack.Forced),
	}
	ctx.Video.Flags = getEbmlTrackFlags(ebmlMeta, "video", 1, baseFlags)

	var settings []string
	if cabac := vTrack.FormatSettingsCABAC; cabac == "Yes" {
		settings = append(settings, "CABAC")
	}

	if ref := vTrack.FormatSettingsRefFrames; ref != "" {
		settings = append(settings, ref+" Ref Frames")
	}

	ctx.Video.Settings = strings.Join(settings, " / ")

	ctx.Video.Library = vTrack.EncodedLibrary
	ctx.Video.LibrarySettings = vTrack.EncodedLibrarySettings

	ctx.Video.Bitrate = fmt.Sprintf("%d kb/s", vTrack.BitRate/1000)
	ctx.Video.Dimensions = fmt.Sprintf("%dx%d", vTrack.Width, vTrack.Height)

	populateVideoAspectAndResolution(ctx, vTrack)
}

func populateVideoAspectAndResolution(ctx *Context, vTrack *mediainfo.Track) {
	// Calculate Aspect Ratio using GCD
	a, b := vTrack.Width, vTrack.Height
	for b != 0 {
		t := b
		b = a % b
		a = t
	}

	if a > 0 {
		ctx.Video.AspectRatio = fmt.Sprintf("%d:%d", vTrack.Width/a, vTrack.Height/a)
	} else if vTrack.DisplayAspectRatio > 0 {
		ctx.Video.AspectRatio = fmt.Sprintf("%.3f", vTrack.DisplayAspectRatio)
	}

	ctx.Video.Resolution = metadata.HeightToResolution(vTrack.Height, vTrack.ScanType, vTrack.FrameRate)
	if ctx.Video.Resolution == "" {
		ctx.Video.Resolution = fmt.Sprintf("%vx%v", vTrack.Width, vTrack.Height)
	}

	if vTrack.FrameRate > 0 {
		ctx.Video.Framerate = fmt.Sprintf("%.3f FPS", vTrack.FrameRate)
	}

	if ctx.Resolution == "" {
		ctx.Resolution = ctx.Video.Resolution
	}
}

func populateAudioContext(ctx *Context, info *mediainfo.MediaInfo, ebmlMeta *matroska.EbmlMetadata) {
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

		ctx.Audio = append(ctx.Audio, audio)
	}
}

func populateTextContext(ctx *Context, info *mediainfo.MediaInfo, ebmlMeta *matroska.EbmlMetadata) {
	for i, tTrack := range getTracksByType(info, "Text") {
		isSDH := strings.Contains(strings.ToLower(tTrack.Title), "sdh") ||
			strings.Contains(strings.ToLower(tTrack.Title), "hearing impaired")

		elemCount := 0
		if tTrack.ElementCount != nil {
			elemCount = *tTrack.ElementCount
		}

		ctx.Subtitles = append(ctx.Subtitles, Subtitle{
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

// StreamingServiceNames maps streaming service tags to their full descriptive names.
var StreamingServiceNames = map[string]string{
	"3SAT":  "3Sat",
	"9NOW":  "9Now",
	"AND":   "Animation Digital Network",
	"AE":    "A&E",
	"AJAZ":  "Al Jazeera English",
	"ALL4":  "All4 (Channel 4)",
	"AMBC":  "ABC (US)",
	"AMC":   "AMC",
	"AMZN":  "Amazon",
	"ANLB":  "AnimeLab",
	"ANPL":  "Animal Planet",
	"AOL":   "AOL",
	"ARD":   "ARD Mediathek",
	"ARDP":  "ARD Plus",
	"ARTE":  "ARTE",
	"AS":    "Adult Swim",
	"ATK":   "America's Test Kitchen",
	"ATV":   "Apple TV (channel content)",
	"ATVP":  "Apple TV+ (original content)",
	"AUBC":  "ABC (AU) iView",
	"BCORE": "Sony Pictures Core",
	"BKPL":  "Blackpills",
	"BNGE":  "Binge",
	"BOOM":  "Boomerang",
	"BRAV":  "BravoTV",
	"CANP":  "Canal+",
	"CBC":   "CBC",
	"CBS":   "CBS",
	"CC":    "Comedy Central",
	"CCGC":  "Comedians in Cars Getting Coffee",
	"CHGD":  "CHRGD",
	"CLBI":  "Club illico",
	"CMAX":  "Cinemax",
	"CMOR":  "C More",
	"CMT":   "Country Music Television",
	"CN":    "Cartoon Network",
	"CNBC":  "CNBC",
	"CNLP":  "Canal+",
	"COOK":  "Cooking Channel",
	"CR":    "Crunchy Roll",
	"CRIT":  "Criterion Channel",
	"CRKL":  "Crackle",
	"CSPN":  "CSpan",
	"CTV":   "CTV",
	"CUR":   "CuriosityStream",
	"CW":    "The CW",
	"CWS":   "CWSeed",
	"DCU":   "DC Universe",
	"DDY":   "Digiturk Dilediğin Yerde",
	"DEST":  "Destination America",
	"DHF":   "Deadhouse Films",
	"DISC":  "Discovery Channel",
	"DIY":   "DIY Network",
	"DOCC":  "Doc Club",
	"DRPO":  "Dropout",
	"DSCP":  "Discovery+",
	"DSKI":  "Daisuki",
	"DSNP":  "Disney+",
	"DSNY":  "Disney",
	"DTV":   "DirecTV Now",
	"EPIX":  "EPIX",
	"ESPN":  "ESPN",
	"ESQ":   "Esquire",
	"ETTV":  "El Trece",
	"ETV":   "E!",
	"FAM":   "Family",
	"FJR":   "Family Jr",
	"FOOD":  "Food Network",
	"FOX":   "Fox",
	"FPT":   "FPT Play",
	"FREE":  "Freeform",
	"FTV":   "France.tv",
	"FUNI":  "Funimation",
	"FXTL":  "Foxtel Now",
	"FYI":   "FYI Network",
	"GC":    "NHL GameCenter",
	"GLBL":  "Global",
	"GLBO":  "Globoplay",
	"GO90":  "go90",
	"HBO":   "HBO",
	"HGTV":  "HGTV",
	"HIDI":  "HIDIVE",
	"HIST":  "History Channel",
	"HLMK":  "Hallmark",
	"HMAX":  "HBO Max",
	"HULU":  "Hulu",
	"ID":    "Investigation Discovery",
	"IFC":   "IFC",
	"IP":    "BBC iPlayer",
	"IT":    "iTunes",
	"ITV":   "ITV",
	"JOYN":  "Joyn",
	"KAYO":  "Kayo Sports",
	"KIKA":  "KiKA",
	"KNOW":  "Knowledge Network",
	"KNPY":  "Kanopy",
	"LIFE":  "Lifetime",
	"LN":    "Loving Nature",
	"MA":    "Movies Anywhere",
	"MAX":   "Max (Warner Bros. Discovery)",
	"MNBC":  "MSNBC",
	"MTOD":  "Motor Trend OnDemand",
	"MTV":   "MTV",
	"NATG":  "National Geographic",
	"NBA":   "NBA League Pass",
	"NBC":   "NBC",
	"NF":    "Netflix",
	"NFL":   "NFL Network",
	"NFLN":  "NFL Now",
	"NICK":  "Nickelodeon",
	"NOW":   "Now (Sky)",
	"NRK":   "Norsk Rikskringkasting",
	"PA":    "Project Alpha",
	"PBS":   "PBS",
	"PBSK":  "PBS Kids",
	"PCOK":  "Peacock",
	"PLAY":  "Google Play",
	"PLUZ":  "Pluzz",
	"PMNT":  "Paramount Network",
	"PMTP":  "Paramount+",
	"POGO":  "PokerGo",
	"PSN":   "Playstation Network",
	"PUHU":  "puhutv",
	"RKTN":  "Rakuten TV",
	"ROKU":  "The Roku Channel",
	"RSTR":  "Rooster Teeth",
	"RTE":   "RTÉ",
	"RTL":   "RTL+",
	"RTLP":  "RTL+",
	"SBS":   "SBS (AU)",
	"SESO":  "Seeso",
	"SHDR":  "Shudder",
	"SHMI":  "Shomi",
	"SHO":   "Showtime",
	"SKST":  "SkyShowtime",
	"SNET":  "Sportsnet",
	"SPIK":  "Spike",
	"SPRT":  "Sprout",
	"STAN":  "Stan",
	"STRP":  "Star+",
	"STZ":   "Starz",
	"SVT":   "Sveriges Television",
	"SWER":  "SwearNet",
	"SYFY":  "SyFy",
	"TBS":   "TBS",
	"TEN":   "TenPlay",
	"TFOU":  "TFOU",
	"TIMV":  "TIMvision",
	"TLC":   "TLC",
	"TOU":   "Ici TOU.TV",
	"TRVL":  "Travel Channel",
	"TUBI":  "TubiTV",
	"TV3":   "TV3 (IE)",
	"TV4":   "TV4 (SE)",
	"TVL":   "TVLand",
	"UFC":   "UFC",
	"UKTV":  "UKTV",
	"UNIV":  "Univision",
	"USAN":  "USA Network",
	"VH1":   "VH1",
	"VIAP":  "Viaplay",
	"VICE":  "Viceland",
	"VLCT":  "Velocity",
	"VMEO":  "Vimeo",
	"VRV":   "VRV",
	"VTRN":  "VET Tv",
	"WME":   "WatchMe",
	"WNET":  "W Network",
	"WOWTV": "WowTV (Sky)",
	"WPU":   "Waipu",
	"WWEN":  "WWE Network",
	"XBOX":  "Xbox Video",
	"YHOO":  "Yahoo",
	"YT":    "YouTube",
	"ZDF":   "ZDF Mediathek",
}

func expandServiceName(tag string) string {
	if name, ok := StreamingServiceNames[strings.ToUpper(tag)]; ok {
		return name
	}

	return tag
}

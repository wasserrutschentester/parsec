package checks

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/metadata/mediainfo"
)

var (
	getMediaInfo = mediainfo.Get

	// titleJunkPatterns flags technical/release metadata noise in the global
	// container title. Shared between checkTitleHygiene and the fix policy.
	titleJunkPatterns = []string{
		`\[.*\]`, // Bracketed info
		`\(.*\)`, // Parenthesized info
		`\b1080p\b`, `\b720p\b`, `\b2160p\b`,
		`\bWEB-DL\b`, `\bBlu-ray\b`, `\bBD\b`,
		`\bx264\b`, `\bx265\b`, `\bHEVC\b`,
	}

	appJunkPatterns = []string{
		`[a-zA-Z]:\\`,            // Windows paths
		`/(home|Users|var|tmp)/`, // Unix paths
		`\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`, // UUID
	}
)

func checkVideoCropping(track matroska.EbmlTrack) *CheckResult {
	if track.Type != "video" {
		return nil
	}

	props := track.Properties

	pixelWidth, pixelHeight := matroska.ParseDimensions(props.PixelDimensions)
	if pixelWidth == 0 || pixelHeight == 0 {
		return nil
	}

	displayWidth, displayHeight := matroska.ParseDimensions(props.DisplayDimensions)
	if displayWidth <= 0 || displayHeight <= 0 {
		return nil
	}

	pixelAR := float64(pixelWidth) / float64(pixelHeight)
	displayAR := float64(displayWidth) / float64(displayHeight)

	// If display AR is wider than pixel AR, but no crop values are set,
	// it might be a "fake" crop or black bars that should be cropped.
	if displayAR > pixelAR+0.01 {
		warning := fmt.Sprintf("resolution-based black bars detected but no MKV crop values set (AR %.2f vs Display AR %.2f)", pixelAR, displayAR)

		return newFailedTrackResult("matroska_video_cropping", "Missing MKV Cropping", "warning", &track, warning)
	}

	return nil
}

func checkTitleHygiene(ebml *matroska.EbmlMetadata, meta *metadata.Metadata) *CheckResult {
	title := ebml.Container.Properties.Title
	if TitleHygieneNeedsFix(title, meta) {
		return &CheckResult{
			Identifier: "matroska_title_hygiene",
			Warning:    "Global Title contains technical metadata",
			Passed:     false,
			Severity:   "warning",
			Actual:     title,
		}
	}

	return nil
}

// TitleHygieneNeedsFix reports whether a container title should be cleared.
// It mirrors checkTitleHygiene's metadata-aware exemption so fix never clears a
// title that check would accept as the parsed/official title.
func TitleHygieneNeedsFix(title string, meta *metadata.Metadata) bool {
	if title == "" {
		return false
	}

	if meta != nil && meta.Title != "" {
		if normalizeForComparison(title) == normalizeForComparison(meta.Title) {
			return false
		}
	}

	return matchesAnyPattern(title, titleJunkPatterns)
}

func checkAppHygiene(ebml *matroska.EbmlMetadata) *CheckResult {
	app := ebml.Container.Properties.WritingApplication
	if app == "" {
		return nil
	}

	if AppHygieneNeedsFix(app) {
		return &CheckResult{
			Identifier: "matroska_app_hygiene",
			Warning:    "Writing Application metadata contains potentially identifiable information",
			Passed:     false,
			Severity:   "warning",
			Actual:     app,
		}
	}

	return nil
}

// AppHygieneNeedsFix reports whether WritingApplication should be cleared.
func AppHygieneNeedsFix(app string) bool {
	if app == "" {
		return false
	}

	return matchesAnyPattern(app, appJunkPatterns)
}

func matchesAnyPattern(value string, patterns []string) bool {
	for _, p := range patterns {
		if regexp.MustCompile("(?i)" + p).MatchString(value) {
			return true
		}
	}

	return false
}

// checkTrueHDCompatibility checks if a Dolby TrueHD audio track is followed by a lossy compatibility track (AC3/EAC3) in the same language.
func checkTrueHDCompatibility(tracks []matroska.EbmlTrack) *CheckResult {
	res := &CheckResult{
		Identifier: "matroska_truehd_compatibility",
		Warning:    "TrueHD track is not followed by a lossy compatibility track",
		Passed:     true,
	}

	for i := range tracks {
		track := &tracks[i]
		if track.Type != "audio" {
			continue
		}

		if !strings.Contains(strings.ToUpper(track.Codec), "A_TRUEHD") {
			continue
		}

		// TrueHD track found. Check next track.
		if i+1 >= len(tracks) {
			res.Passed = false
			res.Severity = "warning"
			res.Tracks = append(res.Tracks, ebmlTrackToResult(track, false, "TrueHD track is the last track and has no lossy compatibility track"))

			continue
		}

		nextTrack := &tracks[i+1]
		if nextTrack.Type != "audio" {
			res.Passed = false
			res.Severity = "warning"
			res.Tracks = append(res.Tracks, ebmlTrackToResult(track, false, "TrueHD track is followed by a non-audio track of type "+nextTrack.Type))

			continue
		}

		if nextTrack.Properties.Language != track.Properties.Language {
			res.Passed = false
			res.Severity = "warning"
			res.Tracks = append(res.Tracks, ebmlTrackToResult(track, false, fmt.Sprintf("TrueHD track is followed by a track with different language: %s (expected %s)", nextTrack.Properties.Language, track.Properties.Language)))

			continue
		}

		nextCodec := strings.ToUpper(nextTrack.Codec)
		if !strings.Contains(nextCodec, "A_AC3") && !strings.Contains(nextCodec, "A_EAC3") {
			res.Passed = false
			res.Severity = "warning"
			res.Tracks = append(res.Tracks, ebmlTrackToResult(track, false, fmt.Sprintf("TrueHD track is followed by an incompatible codec: %s (expected AC3 or EAC3)", nextTrack.Codec)))

			continue
		}
	}

	if !res.Passed {
		return res
	}

	return nil
}

func checkCommentaryChannels(tracks []matroska.EbmlTrack) *CheckResult {
	res := &CheckResult{
		Identifier: "matroska_commentary_channels",
		Warning:    "Commentary audio track has more than 2 channels",
		Passed:     true,
	}

	for i := range tracks {
		track := &tracks[i]

		if track.Type == "audio" && track.Properties.Commentary {
			if track.Properties.AudioChannels > 2 {
				res.Passed = false
				res.Severity = "warning"
				res.Tracks = append(res.Tracks, ebmlTrackToResult(track, false, fmt.Sprintf("Commentary track has %d channels (expected <= 2)", track.Properties.AudioChannels)))
			}
		}
	}

	if !res.Passed {
		return res
	}

	return nil
}

func isLosslessCodec(miTrack *mediainfo.Track) bool {
	codec := metadata.AudioCodecName(miTrack.Format, miTrack.FormatProfile, miTrack.FormatAdditionalFeatures)
	c := strings.ToUpper(codec)

	return strings.Contains(c, "TRUEHD") || strings.Contains(c, "DTS-HD MA") || strings.Contains(c, "FLAC") || strings.Contains(c, "PCM") || strings.Contains(c, "ALAC")
}

func checkCommentaryBitrate(filePath string, tracks []matroska.EbmlTrack) *CheckResult {
	isRemux := strings.Contains(strings.ToUpper(filepath.Base(filePath)), "REMUX")
	if isRemux {
		return nil
	}

	mi, err := getMediaInfo(filePath)
	if err != nil {
		return nil
	}

	miAudioTracks := make(map[string]*mediainfo.Track)

	for i := range mi.Media.Tracks {
		t := &mi.Media.Tracks[i]

		if t.Type == "Audio" {
			miAudioTracks[t.ID] = t
		}
	}

	res := &CheckResult{
		Identifier: "matroska_commentary_bitrate",
		Warning:    "Commentary audio track bitrate exceeds 128 kbps",
		Passed:     true,
	}

	for i := range tracks {
		if tr := verifyCommentaryTrackBitrate(&tracks[i], miAudioTracks); tr != nil {
			res.Passed = false
			res.Severity = "warning"
			res.Tracks = append(res.Tracks, *tr)
		}
	}

	if !res.Passed {
		return res
	}

	return nil
}

func verifyCommentaryTrackBitrate(track *matroska.EbmlTrack, miAudioTracks map[string]*mediainfo.Track) *TrackCheckResult {
	if track.Type != "audio" || !track.Properties.Commentary {
		return nil
	}

	idStr := strconv.Itoa(track.Properties.Number)

	miTrack, ok := miAudioTracks[idStr]
	if !ok {
		return nil
	}

	if isLosslessCodec(miTrack) {
		return nil
	}

	if miTrack.BitRate > 0 && miTrack.BitRate > 128000 {
		bitrateKbps := float64(miTrack.BitRate) / 1000.0
		tr := ebmlTrackToResult(track, false, fmt.Sprintf("Commentary track bitrate is %.1f kbps (expected <= 128 kbps)", bitrateKbps))

		return &tr
	}

	return nil
}

var (
	commentaryPrefixRegex = regexp.MustCompile(`^(?:.*/\s*)?(?:Commentary by|Isolated score with commentary by)\b`)
	commentaryByRegex     = regexp.MustCompile(`(?i)commentary by`)
	isolatedScoreRegex    = regexp.MustCompile(`(?i)isolated score`)
)

func checkCommentaryPrefix(tracks []matroska.EbmlTrack) *CheckResult {
	res := &CheckResult{
		Identifier: "matroska_commentary_prefix",
		Warning:    "Commentary track name does not start with a standard prefix",
		Passed:     true,
	}

	for i := range tracks {
		track := &tracks[i]

		if track.Properties.Commentary {
			name := track.Properties.Name
			if name == "" {
				res.Passed = false
				res.Severity = "warning"
				res.Tracks = append(res.Tracks, ebmlTrackToResult(track, false, "Commentary track has no name"))

				continue
			}

			if !commentaryPrefixRegex.MatchString(name) {
				res.Passed = false
				res.Severity = "warning"
				res.Tracks = append(res.Tracks, ebmlTrackToResult(track, false, fmt.Sprintf("Track name %q does not start with standard prefix (e.g., \"Commentary by ...\")", name)))
			}
		}
	}

	if !res.Passed {
		return res
	}

	return nil
}

// extractCommentaryCoreRaw returns the core identifying part of a commentary
// track name with original case preserved and without SDH stripping.
func ExtractCommentaryCoreOriginalCase(name string) string {
	if loc := commentaryByRegex.FindStringIndex(name); loc != nil {
		name = name[loc[0]:]
	} else if loc := isolatedScoreRegex.FindStringIndex(name); loc != nil {
		name = name[loc[0]:]
	} else {
		if _, after, ok := strings.Cut(name, "/"); ok {
			name = after
		}
	}

	return strings.TrimSpace(name)
}

func ExtractCommentaryCore(name string) string {
	name = ExtractCommentaryCoreOriginalCase(name)
	name = strings.ReplaceAll(name, "(SDH)", "")
	name = strings.ReplaceAll(name, "[SDH]", "")
	name = strings.ReplaceAll(name, "SDH", "")

	return strings.ToLower(strings.TrimSpace(name))
}

func checkCommentaryPairing(tracks []matroska.EbmlTrack) *CheckResult {
	res := &CheckResult{
		Identifier: "matroska_commentary_pairing",
		Warning:    "Commentary subtitle track name does not match any audio commentary track name",
		Passed:     true,
	}

	var audioCommentaries []string

	for i := range tracks {
		track := &tracks[i]

		if track.Type == "audio" && track.Properties.Commentary {
			core := ExtractCommentaryCore(track.Properties.Name)
			audioCommentaries = append(audioCommentaries, core)
		}
	}

	for i := range tracks {
		track := &tracks[i]

		if track.Type == "subtitles" && track.Properties.Commentary {
			core := ExtractCommentaryCore(track.Properties.Name)

			if !slices.Contains(audioCommentaries, core) {
				res.Passed = false
				res.Severity = "warning"
				res.Tracks = append(res.Tracks, ebmlTrackToResult(track, false, fmt.Sprintf("Commentary subtitle %q does not match any audio commentary track", track.Properties.Name)))
			}
		}
	}

	if !res.Passed {
		return res
	}

	return nil
}

// CreationTimeTagKeys lists the raw Matroska tag Name values that
// matroska_creation_time_privacy treats as an encode/creation-time privacy
// concern when found in a track's or the file's global Tags. Shared with
// internal/correct's tag-stripping fix so the two can never disagree about
// which tag names carry creation-time metadata.
var CreationTimeTagKeys = []string{
	"creation_time",
	"ENCODED_DATE",
	"DATE_ENCODED",
	"DATE_TAGGED",
	"_STATISTICS_WRITING_DATE_UTC",
	"DATE",
}

// ContainerCreationTimeNeedsFix reports whether the Segment-level DateUTC
// (and its derived DateLocal display value) should be cleared, matching
// matroska_creation_time_privacy's container-level criteria.
func ContainerCreationTimeNeedsFix(ebml *matroska.EbmlMetadata) bool {
	return ebml.Container.Properties.DateUtc != "" || ebml.Container.Properties.DateLocal != ""
}

// emptyTagRegex matches a <Tag> element left with a Targets child and no
// remaining Simple entries, non-greedy up to the first closing Targets tag
// (Targets elements don't nest, so this is unambiguous).
var emptyTagRegex = regexp.MustCompile(`(?is)<Tag>\s*(<Targets\s*/>|<Targets>.*?</Targets>)\s*</Tag>`)

// StripCreationTimeTags removes <Simple> tag entries (global or per-track)
// whose <Name> is one of CreationTimeTagKeys from tagsXML (as extracted by
// matroska.ExtractTagsXML), along with any <Tag> block left with no Simple
// children as a result. Matching is done on the raw XML text rather than a
// full parse and remarshal: the Tags schema's Targets element can carry
// TrackUID/EditionUID/ChapterUID/AttachmentUID children this package
// otherwise doesn't model, and a lossy round trip through an incomplete
// struct could silently drop a tag's association with its track. Returns
// the original content unchanged and a nil name list when nothing matched.
func StripCreationTimeTags(tagsXML []byte) ([]byte, []string) {
	content := tagsXML

	var removed []string

	for _, key := range CreationTimeTagKeys {
		re := regexp.MustCompile(`(?is)\s*<Simple>\s*<Name>` + regexp.QuoteMeta(key) + `</Name>.*?</Simple>`)
		if re.Match(content) {
			removed = append(removed, key)
			content = re.ReplaceAll(content, nil)
		}
	}

	if len(removed) == 0 {
		return tagsXML, nil
	}

	content = emptyTagRegex.ReplaceAll(content, nil)

	return content, removed
}

func checkCreationTimePrivacy(filePath string, ebml *matroska.EbmlMetadata) *CheckResult {
	res := &CheckResult{
		Identifier: "matroska_creation_time_privacy",
		Warning:    "Privacy concern: file contains creation/encode time metadata",
		Passed:     true,
		Severity:   "info",
	}

	if ebml.Container.Properties.DateUtc != "" {
		res.Passed = false
		res.Actual = "DateUTC: " + ebml.Container.Properties.DateUtc
	}

	if ebml.Container.Properties.DateLocal != "" {
		res.Passed = false
		if res.Actual != "" {
			res.Actual += "; "
		}

		res.Actual += "DateLocal: " + ebml.Container.Properties.DateLocal
	}

	if mi, err := getMediaInfo(filePath); err == nil {
		appendMediaInfoCreationTimePrivacy(mi, res)
	}

	if !res.Passed {
		return res
	}

	return nil
}

func appendMediaInfoCreationTimePrivacy(mi *mediainfo.MediaInfo, res *CheckResult) {
	for i := range mi.Media.Tracks {
		t := &mi.Media.Tracks[i]

		fields := map[string]string{
			"Encoded_Date": t.EncodedDate,
			"Tagged_Date":  t.TaggedDate,
		}

		for _, key := range CreationTimeTagKeys {
			fields[key] = t.Extra.GetString(key)
		}

		for k, v := range fields {
			if v != "" {
				res.Passed = false
				if res.Actual != "" {
					res.Actual += "; "
				}

				prefix := ""
				if t.Type != "General" {
					prefix = t.Type + " "
				}

				res.Actual += prefix + k + ": " + v
			}
		}
	}
}

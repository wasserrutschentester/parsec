package checks

import (
	"fmt"
	"regexp"
	"strings"

	"codeberg.org/n0ne/parsec/internal/config"
	"codeberg.org/n0ne/parsec/internal/metadata/matroska"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

var (
	dtsRegex        = regexp.MustCompile(`\bDTS\b`)
	adRegex         = regexp.MustCompile(`\bAD\b`)
	wordSplitRegex  = regexp.MustCompile(`[\s/.,;()]+`)
	commonLangNames = map[string]string{
		"english": "en", "german": "de", "french": "fr", "spanish": "es",
		"italian": "it", "japanese": "ja", "chinese": "zh", "korean": "ko",
		"russian": "ru", "portuguese": "pt", "dutch": "nl", "polish": "pl",
		"swedish": "sv", "danish": "da", "norwegian": "no", "finnish": "fi",
		"arabic": "ar", "hindi": "hi", "turkish": "tr", "thai": "th",
		"vietnamese": "vi", "indonesian": "id", "hebrew": "he", "czech": "cs",
		"hungarian": "hu", "romanian": "ro", "greek": "el", "bulgarian": "bg",
		"deutsch": "de", "français": "fr", "francais": "fr", "español": "es",
		"espanol": "es", "italiano": "it", "日本語": "ja", "中文": "zh",
		"한국어": "ko", "русский": "ru",
	}

	junkKeywords = []string{"STEREO", "SURROUND", "EXTERNAL", "UPLOADED", "ENCODED"}
	simpleCodecs = []string{"AC3", "AAC", "E-AC3", "EAC3", "FLAC"}
)

const (
	priorityPreferred = 1000
	priorityOriginal  = 2000
	priorityMul       = 3000
	priorityEnglish   = 4000
	priorityOther     = 5000

	propScoreCommentary  = 30
	propScoreAD          = 10
	propScoreDescription = 20
	propScoreForced      = 0
	propScoreSDH         = 20
	propScoreStandard    = 10
)

type trackResultAggregator struct {
	aggregated map[string]*CheckResult
}

func newTrackResultAggregator() *trackResultAggregator {
	return &trackResultAggregator{
		aggregated: make(map[string]*CheckResult),
	}
}

func (a *trackResultAggregator) Add(res *CheckResult) {
	if res == nil {
		return
	}
	target, ok := a.aggregated[res.Identifier]
	if !ok {
		target = &CheckResult{
			Identifier:  res.Identifier,
			Description: res.Description,
			Passed:      true,
		}
		a.aggregated[res.Identifier] = target
	}
	target.Passed = false
	target.Severity = res.Severity
	target.Tracks = append(target.Tracks, res.Tracks...)
}

func newFailedTrackResult(id, desc, severity string, track *matroska.EbmlTrack, warning string) *CheckResult {
	return &CheckResult{
		Identifier:  id,
		Description: desc,
		Passed:      false,
		Severity:    severity,
		Tracks:      []TrackCheckResult{ebmlTrackToResult(track, false, warning)},
	}
}

func RunMatroskaChecks(filePath string) []CheckResult {
	ebml, err := matroska.GetEbmlMetadata(filePath)
	if err != nil {
		return []CheckResult{{
			Identifier:  "matroska_ebml_error",
			Description: "Ability to read EBML metadata",
			Passed:      false,
			Severity:    "error",
			Warning:     fmt.Sprintf("Error getting EBML metadata: %v", err),
		}}
	}

	return RunTrackChecks(ebml.Tracks)
}

func RunTrackChecks(tracks []matroska.EbmlTrack) []CheckResult {
	var results []CheckResult
	var lastAudioPriority, lastSubPriority int
	seenTracks := make(map[string]bool)
	seenAudioLangs := make(map[string]bool)
	seenSubLangs := make(map[string]bool)

	agg := newTrackResultAggregator()

	audioCounts, subCounts := getTrackCounts(tracks)
	langHasOriginalFlag := getOriginalLanguageMap(tracks)

	for i := range tracks {
		track := &tracks[i]
		if !isRelevantTrack(*track) {
			continue
		}

		if config.IsCheckEnabled("matroska_language_tag") || config.IsCheckEnabled("matroska_multi_lang") {
			agg.Add(validateTrackBasics(*track))
		}

		if config.IsCheckEnabled("matroska_name_quality") {
			agg.Add(checkTrackNameQuality(*track))
		}

		if config.IsCheckEnabled("matroska_name_codecs") {
			agg.Add(checkTrackNameCodecs(*track))
		}

		if config.IsCheckEnabled("matroska_name_redundant_lang") {
			agg.Add(checkTrackNameRedundantLang(*track))
		}

		if config.IsCheckEnabled("matroska_original_language") {
			agg.Add(checkOriginalLanguageConsistency(*track, langHasOriginalFlag))
		}

		if config.IsCheckEnabled("matroska_duplicate_tracks") {
			agg.Add(checkDuplicateTracks(*track, seenTracks))
		}

		if config.IsCheckEnabled("matroska_name_keywords") {
			agg.Add(checkNameKeywords(*track))
		}

		if config.IsCheckEnabled("matroska_default_flags") {
			agg.Add(checkDefaultFlags(*track, audioCounts, subCounts, seenAudioLangs, seenSubLangs))
		}

		if config.IsCheckEnabled("matroska_subtitle_format") {
			agg.Add(checkSubtitleFormat(*track))
		}

		if config.IsCheckEnabled("matroska_track_order") {
			priority := getTrackPriority(*track)
			if track.Type == "audio" {
				agg.Add(checkTrackOrder(track, priority, &lastAudioPriority, "Audio tracks order consistency"))
			} else if track.Type == "subtitles" {
				agg.Add(checkTrackOrder(track, priority, &lastSubPriority, "Subtitle tracks order consistency"))
			}
		}
	}

	// Convert aggregated map to slice in stable order
	ids := []string{
		"matroska_language_tag",
		"matroska_multi_lang",
		"matroska_name_quality",
		"matroska_name_codecs",
		"matroska_name_redundant_lang",
		"matroska_original_language",
		"matroska_duplicate_tracks",
		"matroska_name_keywords",
		"matroska_default_flags",
		"matroska_subtitle_format",
		"matroska_track_order",
	}
	for _, id := range ids {
		if res, ok := agg.aggregated[id]; ok && !res.Passed {
			results = append(results, *res)
		}
	}

	return results
}

func checkTrackOrder(track *matroska.EbmlTrack, priority int, lastPriority *int, description string) *CheckResult {
	if priority < *lastPriority {
		warning := fmt.Sprintf("score: %d, prev: %d", priority, *lastPriority)
		*lastPriority = priority
		return newFailedTrackResult("matroska_track_order", description, "warning", track, warning)
	}
	*lastPriority = priority
	return nil
}

func getOriginalLanguageMap(tracks []matroska.EbmlTrack) map[string]bool {
	langHasOriginalFlag := make(map[string]bool)
	for _, track := range tracks {
		if isRelevantTrack(track) && track.Properties.OriginalLanguage {
			langHasOriginalFlag[track.Properties.Language] = true
		}
	}
	return langHasOriginalFlag
}

func ebmlTrackToResult(t *matroska.EbmlTrack, passed bool, warning string) TrackCheckResult {
	return TrackCheckResult{
		ID:        fmt.Sprintf("%d", t.Properties.Number),
		Type:      t.Type,
		TypeOrder: t.TypeOrder,
		Codec:     t.Codec,
		Name:      t.Properties.Name,
		Language:  t.Properties.Language,
		Flags:     ebmlGetFlagsSlice(t),
		Passed:    passed,
		Warning:   warning,
	}
}

func ebmlGetFlagsSlice(track *matroska.EbmlTrack) []string {
	var flags []string
	if track.Properties.Default {
		flags = append(flags, "Default")
	}
	if track.Properties.Forced {
		flags = append(flags, "Forced")
	}
	if track.Properties.HearingImpaired {
		flags = append(flags, "Hearing Impaired")
	}
	if track.Properties.VisualImpaired {
		flags = append(flags, "Visual Impaired")
	}
	if track.Properties.Commentary {
		flags = append(flags, "Commentary")
	}
	if track.Properties.OriginalLanguage {
		flags = append(flags, "Original")
	}
	return flags
}

func checkTrackNameQuality(track matroska.EbmlTrack) *CheckResult {
	nameUpper := strings.ToUpper(track.Properties.Name)
	for _, junk := range junkKeywords {
		if strings.Contains(nameUpper, junk) {
			warning := fmt.Sprintf("junk keyword '%s' in Name", junk)
			return newFailedTrackResult("matroska_name_quality", "Track Name contains junk keywords", "info", &track, warning)
		}
	}
	return nil
}

func checkTrackNameCodecs(track matroska.EbmlTrack) *CheckResult {
	nameUpper := strings.ToUpper(track.Properties.Name)
	for _, codec := range simpleCodecs {
		if strings.Contains(nameUpper, codec) {
			warning := fmt.Sprintf("simple codec '%s' in Name", codec)
			return newFailedTrackResult("matroska_name_codecs", "Track Name contains simple codec", "info", &track, warning)
		}
	}
	// Special handling for DTS (allow DTS-HD, DTS:X etc)
	if strings.Contains(nameUpper, "DTS") && !strings.Contains(nameUpper, "DTS-HD") && !strings.Contains(nameUpper, "DTS:X") && !strings.Contains(nameUpper, "DTS-ES") {
		if dtsRegex.MatchString(nameUpper) {
			return newFailedTrackResult("matroska_name_codecs", "Track Name contains simple codec", "info", &track, "simple codec 'DTS' in Name")
		}
	}
	return nil
}

func checkTrackNameRedundantLang(track matroska.EbmlTrack) *CheckResult {
	if isRedundantLanguageName(track.Properties.Name, track.Properties.Language) {
		return newFailedTrackResult("matroska_name_redundant_lang", "Track Name contains redundant language name", "info", &track, "redundant language in Name")
	}
	return nil
}

func isRedundantLanguageName(name, trackLang string) bool {
	tag := language.Make(trackLang)
	base, _ := tag.Base()
	target := base.String()

	for _, word := range tokenizeTrackName(name) {
		if getLanguageCodeFromName(word) == target {
			return true
		}
	}
	return false
}

func countLanguagesInString(name string) int {
	count := 0
	for _, word := range tokenizeTrackName(name) {
		if isLanguageName(word) {
			count++
		}
	}
	return count
}

func tokenizeTrackName(name string) []string {
	if name == "" {
		return nil
	}
	return wordSplitRegex.Split(name, -1)
}

func isLanguageName(word string) bool {
	return getLanguageCodeFromName(word) != ""
}

func getLanguageCodeFromName(word string) string {
	if len(word) <= 3 {
		return ""
	}

	wordLower := strings.ToLower(word)

	if base := commonLangNames[wordLower]; base != "" {
		return base
	}

	tag, err := language.Parse(word)
	if err == nil {
		langName := display.English.Languages().Name(tag)
		if strings.EqualFold(langName, word) {
			base, _ := tag.Base()
			return base.String()
		}
	}
	return ""
}

func checkDefaultFlags(track matroska.EbmlTrack, audioCounts, subCounts map[string]int, seenAudioLangs, seenSubLangs map[string]bool) *CheckResult {
	props := track.Properties
	shouldBeDefault := false

	// Specialized tracks MUST NEVER be default
	isSpecialized := props.Forced || props.Commentary || props.VisualImpaired || props.HearingImpaired || props.TextDescriptions

	if !isSpecialized {
		if track.Type == "audio" {
			if !seenAudioLangs[props.Language] {
				shouldBeDefault = true
				seenAudioLangs[props.Language] = true
			}
			// Relaxation: if only one track in this language, it is fine to not set the default flag
			if audioCounts[props.Language] == 1 && !props.Default {
				shouldBeDefault = false
			}
		} else if track.Type == "subtitles" {
			if !seenSubLangs[props.Language] {
				shouldBeDefault = true
				seenSubLangs[props.Language] = true
			}
			// Relaxation: if only one track in this language, it is fine to not set the default flag
			if subCounts[props.Language] == 1 && !props.Default {
				shouldBeDefault = false
			}
		}
	}

	if props.Default != shouldBeDefault {
		var warning string
		if shouldBeDefault {
			warning = fmt.Sprintf("[+] first standard track for %s", props.Language)
		} else if isSpecialized {
			warning = "[-] specialized track"
		} else {
			warning = fmt.Sprintf("[-] redundant standard track for %s", props.Language)
		}

		return newFailedTrackResult("matroska_default_flags", "Proper Default flag assignment", "warning", &track, warning)
	}

	return nil
}

func checkSubtitleFormat(track matroska.EbmlTrack) *CheckResult {
	codec := track.Codec
	if track.Type == "subtitles" && track.Properties.TextSubtitles && !strings.Contains(codec, "SRT") {
		warning := fmt.Sprintf("text-based but codec is %s", codec)
		return newFailedTrackResult("matroska_subtitle_format", "Text subtitles should be in SRT format", "warning", &track, warning)
	}
	return nil
}

func getTrackCounts(tracks []matroska.EbmlTrack) (audio, sub map[string]int) {
	audio = make(map[string]int)
	sub = make(map[string]int)
	for _, track := range tracks {
		if !isRelevantTrack(track) {
			continue
		}
		if track.Type == "audio" {
			audio[track.Properties.Language]++
		} else if track.Type == "subtitles" {
			sub[track.Properties.Language]++
		}
	}
	return audio, sub
}

func isRelevantTrack(track matroska.EbmlTrack) bool {
	return track.Type == "audio" || track.Type == "subtitles"
}

func validateTrackBasics(track matroska.EbmlTrack) *CheckResult {
	lang := track.Properties.Language
	tag := language.Make(lang)
	if config.IsCheckEnabled("matroska_language_tag") && tag == language.Und {
		return newFailedTrackResult("matroska_language_tag", "Track has valid language tag", "warning", &track, "missing or invalid language tag")
	}
	if config.IsCheckEnabled("matroska_multi_lang") && tag == language.Make("mul") && track.Properties.Name == "" {
		return newFailedTrackResult("matroska_multi_lang", "Multi-language track must have a Name", "warning", &track, "missing Name for 'mul' language")
	}
	return nil
}

func checkOriginalLanguageConsistency(track matroska.EbmlTrack, langHasOriginalFlag map[string]bool) *CheckResult {
	lang := track.Properties.Language
	if langHasOriginalFlag[lang] && !track.Properties.OriginalLanguage {
		return newFailedTrackResult("matroska_original_language", "Consistency of Original language flag", "info", &track, "missing Original flag")
	}
	return nil
}

func checkDuplicateTracks(track matroska.EbmlTrack, seenTracks map[string]bool) *CheckResult {
	props := track.Properties
	trackKey := fmt.Sprintf("%s-%s-%t-%t-%t-%t-%t-%t-%s",
		track.Type, props.Language, props.Default, props.Forced,
		props.HearingImpaired, props.VisualImpaired,
		props.Commentary, props.OriginalLanguage, props.Name)
	if seenTracks[trackKey] {
		return newFailedTrackResult("matroska_duplicate_tracks", "No duplicate tracks", "warning", &track, "duplicate track")
	}
	seenTracks[trackKey] = true
	return nil
}

func checkNameKeywords(track matroska.EbmlTrack) *CheckResult {
	props := track.Properties
	nameUpper := strings.ToUpper(props.Name)

	if res := checkFlagKeywordResult(track, props.HearingImpaired, "HearingImpaired", "SDH", strings.Contains(nameUpper, "SDH")); res != nil {
		return res
	}

	if res := checkFlagKeywordResult(track, props.Forced, "Forced", "Forced", strings.Contains(nameUpper, "FORCED")); res != nil {
		return res
	}

	if res := checkFlagKeywordResult(track, props.Commentary, "Commentary", "Commentary", strings.Contains(nameUpper, "COMMENTARY")); res != nil {
		return res
	}

	hasVIKeyword := strings.Contains(nameUpper, "DESCRIPTIVE") || strings.Contains(nameUpper, "DESCRIPTION") || adRegex.MatchString(nameUpper)
	if res := checkFlagKeywordResult(track, props.VisualImpaired, "VisualImpaired", "Descriptive', 'Description', or 'AD", hasVIKeyword); res != nil {
		return res
	}

	if props.Language == "mul" {
		if countLanguagesInString(props.Name) < 2 {
			return newFailedTrackResult("matroska_name_keywords", "Multi-language track contains language names in Name field", "warning", &track, "'mul' but Name has <2 language names")
		}
	}

	return nil
}

func checkFlagKeywordResult(track matroska.EbmlTrack, flag bool, flagName, keywordStr string, hasKeyword bool) *CheckResult {
	if flag && !hasKeyword {
		warning := fmt.Sprintf("is %s but Name missing '%s'", flagName, keywordStr)
		return newFailedTrackResult("matroska_name_keywords", "Track Name contains relevant flag keywords", "info", &track, warning)
	}
	if !flag && hasKeyword {
		warning := fmt.Sprintf("'%s' in Name but no %s flag", keywordStr, flagName)
		return newFailedTrackResult("matroska_name_keywords", "Track Name contains relevant flag keywords", "info", &track, warning)
	}
	return nil
}

func getTrackPriority(track matroska.EbmlTrack) int {
	lang := track.Properties.Language
	tag := language.Make(lang)
	prefTag := language.Make(config.GetPreferredLanguage())

	langScore := priorityOther
	if tag == prefTag {
		langScore = priorityPreferred
	} else if track.Properties.OriginalLanguage {
		langScore = priorityOriginal
	} else if tag == language.Make("mul") {
		langScore = priorityMul
	} else if tag == language.English {
		langScore = priorityEnglish
	} else {
		base, _ := tag.Base()
		s := base.String()
		if len(s) >= 2 {
			langScore += int(s[0]-'a')*100 + int(s[1]-'a')*10
			if len(s) >= 3 {
				langScore += int(s[2] - 'a')
			}
		}
	}

	propertyScore := 0
	if track.Type == "audio" {
		if track.Properties.Commentary {
			propertyScore = propScoreCommentary
		} else if track.Properties.VisualImpaired {
			propertyScore = propScoreAD
		} else if track.Properties.TextDescriptions {
			propertyScore = propScoreDescription
		}
	} else if track.Type == "subtitles" {
		if track.Properties.Forced {
			propertyScore = propScoreForced
		} else if track.Properties.HearingImpaired {
			propertyScore = propScoreSDH
		} else {
			propertyScore = propScoreStandard
		}
	}

	return langScore + propertyScore
}

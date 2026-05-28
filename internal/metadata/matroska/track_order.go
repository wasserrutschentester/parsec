package matroska

import (
	"fmt"
	"regexp"
	"strings"

	"codeberg.org/n0ne/parsec/internal/checks"
	"codeberg.org/n0ne/parsec/internal/config"
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
)

func VerifyTrackOrder(tracks []EbmlTrack) []checks.CheckResult {
	var results []checks.CheckResult
	var lastAudioPriority, lastSubPriority int
	seenTracks := make(map[string]bool)
	langHasOriginalFlag := make(map[string]bool)

	// Aggregators for per-track checks
	aggregated := make(map[string]*checks.CheckResult)
	mergeResult := func(res *checks.CheckResult) {
		if res == nil {
			return
		}
		target, ok := aggregated[res.Identifier]
		if !ok {
			target = &checks.CheckResult{
				Identifier:  res.Identifier,
				Description: res.Description,
				Passed:      true,
			}
			aggregated[res.Identifier] = target
		}
		target.Passed = false
		target.Severity = res.Severity
		target.Tracks = append(target.Tracks, res.Tracks...)
	}

	// First pass for consistency info
	for _, track := range tracks {
		if isRelevantTrack(track) && track.Properties.OriginalLanguage {
			langHasOriginalFlag[track.Properties.Language] = true
		}
	}

	for i := range tracks {
		track := &tracks[i]
		if !isRelevantTrack(*track) {
			continue
		}

		if config.IsCheckEnabled("matroska_language_tag") || config.IsCheckEnabled("matroska_multi_lang") {
			mergeResult(validateTrackBasics(*track))
		}

		if config.IsCheckEnabled("matroska_name_quality") {
			mergeResult(checkTrackNameQuality(*track))
		}

		if config.IsCheckEnabled("matroska_name_codecs") {
			mergeResult(checkTrackNameCodecs(*track))
		}

		if config.IsCheckEnabled("matroska_name_redundant_lang") {
			mergeResult(checkTrackNameRedundantLang(*track))
		}

		if config.IsCheckEnabled("matroska_original_language") {
			mergeResult(checkOriginalLanguageConsistency(*track, langHasOriginalFlag))
		}

		if config.IsCheckEnabled("matroska_duplicate_tracks") {
			mergeResult(checkDuplicateTracks(*track, seenTracks))
		}

		if config.IsCheckEnabled("matroska_name_keywords") {
			mergeResult(checkNameKeywords(*track))
		}

		if config.IsCheckEnabled("matroska_track_order") {
			priority := getTrackPriority(*track)
			if track.Type == "audio" {
				if priority < lastAudioPriority {
					warning := fmt.Sprintf("score: %d, prev: %d", priority, lastAudioPriority)
					mergeResult(&checks.CheckResult{
						Identifier:  "matroska_track_order",
						Description: "Audio tracks order consistency",
						Severity:    "warning",
						Tracks:      []checks.TrackCheckResult{trackToResult(track, false, warning)},
					})
				}
				lastAudioPriority = priority
			} else if track.Type == "subtitles" {
				if priority < lastSubPriority {
					warning := fmt.Sprintf("score: %d, prev: %d", priority, lastSubPriority)
					mergeResult(&checks.CheckResult{
						Identifier:  "matroska_track_order",
						Description: "Subtitle tracks order consistency",
						Severity:    "warning",
						Tracks:      []checks.TrackCheckResult{trackToResult(track, false, warning)},
					})
				}
				lastSubPriority = priority
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
		"matroska_track_order",
	}
	for _, id := range ids {
		if res, ok := aggregated[id]; ok && !res.Passed {
			results = append(results, *res)
		}
	}

	return results
}

func trackToResult(t *EbmlTrack, passed bool, warning string) checks.TrackCheckResult {
	return checks.TrackCheckResult{
		ID:        fmt.Sprintf("%d", t.Properties.Number),
		Type:      t.Type,
		TypeOrder: t.TypeOrder,
		Codec:     t.Codec,
		Name:      t.Properties.Name,
		Language:  t.Properties.Language,
		Flags:     t.getFlagsSlice(),
		Passed:    passed,
		Warning:   warning,
	}
}

func (track *EbmlTrack) getFlagsSlice() []string {
	var flags []string
	if track.Properties.Default {
		flags = append(flags, "Default")
	}
	if track.Properties.Forced {
		flags = append(flags, "Forced")
	}
	if track.Properties.HearingImpaired {
		flags = append(flags, "SDH")
	}
	if track.Properties.VisualImpaired {
		flags = append(flags, "AD")
	}
	if track.Properties.Commentary {
		flags = append(flags, "Commentary")
	}
	if track.Properties.OriginalLanguage {
		flags = append(flags, "Original")
	}
	return flags
}

func checkTrackNameQuality(track EbmlTrack) *checks.CheckResult {
	junkKeywords := []string{"STEREO", "SURROUND", "EXTERNAL", "UPLOADED", "ENCODED"}
	nameUpper := strings.ToUpper(track.Properties.Name)
	for _, junk := range junkKeywords {
		if strings.Contains(nameUpper, junk) {
			shortWarning := fmt.Sprintf("junk keyword '%s' in Name", junk)
			return &checks.CheckResult{
				Identifier:  "matroska_name_quality",
				Description: "Track Name contains junk keywords",
				Severity:    "info",
				Tracks:      []checks.TrackCheckResult{trackToResult(&track, false, shortWarning)},
			}
		}
	}
	return nil
}

func checkTrackNameCodecs(track EbmlTrack) *checks.CheckResult {
	nameUpper := strings.ToUpper(track.Properties.Name)
	// Simple Codecs
	simpleCodecs := []string{"AC3", "AAC", "E-AC3", "EAC3", "FLAC"}
	for _, codec := range simpleCodecs {
		if strings.Contains(nameUpper, codec) {
			shortWarning := fmt.Sprintf("simple codec '%s' in Name", codec)
			return &checks.CheckResult{
				Identifier:  "matroska_name_codecs",
				Description: "Track Name contains simple codec",
				Severity:    "info",
				Tracks:      []checks.TrackCheckResult{trackToResult(&track, false, shortWarning)},
			}
		}
	}
	// Special handling for DTS (allow DTS-HD, DTS:X etc)
	if strings.Contains(nameUpper, "DTS") && !strings.Contains(nameUpper, "DTS-HD") && !strings.Contains(nameUpper, "DTS:X") && !strings.Contains(nameUpper, "DTS-ES") {
		if dtsRegex.MatchString(nameUpper) {
			shortWarning := "simple codec 'DTS' in Name"
			return &checks.CheckResult{
				Identifier:  "matroska_name_codecs",
				Description: "Track Name contains simple codec",
				Severity:    "info",
				Tracks:      []checks.TrackCheckResult{trackToResult(&track, false, shortWarning)},
			}
		}
	}
	return nil
}

func checkTrackNameRedundantLang(track EbmlTrack) *checks.CheckResult {
	if isRedundantLanguageName(track.Properties.Name, track.Properties.Language) {
		shortWarning := "redundant language in Name"
		return &checks.CheckResult{
			Identifier:  "matroska_name_redundant_lang",
			Description: "Track Name contains redundant language name",
			Severity:    "info",
			Tracks:      []checks.TrackCheckResult{trackToResult(&track, false, shortWarning)},
		}
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

func CheckDefaultFlags(tracks []EbmlTrack) []checks.CheckResult {
	var results []checks.CheckResult
	seenAudioLangs := make(map[string]bool)
	seenSubLangs := make(map[string]bool)
	audioLangCount := make(map[string]int)
	subLangCount := make(map[string]int)

	for _, track := range tracks {
		if !isRelevantTrack(track) {
			continue
		}
		if track.Type == "audio" {
			audioLangCount[track.Properties.Language]++
		} else if track.Type == "subtitles" {
			subLangCount[track.Properties.Language]++
		}
	}

	res := checks.CheckResult{
		Identifier:  "matroska_default_flags",
		Description: "Proper Default flag assignment",
		Passed:      true,
	}

	for i := range tracks {
		track := &tracks[i]
		if !isRelevantTrack(*track) {
			continue
		}

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
				if audioLangCount[props.Language] == 1 && !props.Default {
					shouldBeDefault = false
				}
			} else if track.Type == "subtitles" {
				if !seenSubLangs[props.Language] {
					shouldBeDefault = true
					seenSubLangs[props.Language] = true
				}
				// Relaxation: if only one track in this language, it is fine to not set the default flag
				if subLangCount[props.Language] == 1 && !props.Default {
					shouldBeDefault = false
				}
			}
		}

		if props.Default != shouldBeDefault {
			res.Passed = false
			res.Severity = "warning"

			var warning string
			if shouldBeDefault {
				warning = fmt.Sprintf("[+] first standard track for %s", props.Language)
			} else if isSpecialized {
				warning = "[-] specialized track"
			} else {
				warning = fmt.Sprintf("[-] redundant standard track for %s", props.Language)
			}

			res.Tracks = append(res.Tracks, trackToResult(track, false, warning))
		}
	}

	if !res.Passed {
		results = append(results, res)
	}
	return results
}

func CheckSubtitleFormat(tracks []EbmlTrack) []checks.CheckResult {
	res := checks.CheckResult{
		Identifier:  "matroska_subtitle_format",
		Description: "Text subtitles should be in SRT format",
		Passed:      true,
	}

	for i := range tracks {
		track := &tracks[i]
		codec := track.Codec
		if track.Type == "subtitles" && track.Properties.TextSubtitles && !strings.Contains(codec, "SRT") {
			warning := fmt.Sprintf("text-based but codec is %s", codec)
			res.Passed = false
			res.Severity = "warning"
			res.Tracks = append(res.Tracks, trackToResult(track, false, warning))
		}
	}

	if !res.Passed {
		return []checks.CheckResult{res}
	}
	return nil
}

func (track *EbmlTrack) getFlags() string {
	if track == nil {
		return "None"
	}
	props := track.Properties
	flags := ""
	if props.Default {
		flags += " [Default]"
	}
	if props.Forced {
		flags += " [Forced]"
	}
	if props.HearingImpaired {
		flags += " [Hearing Impaired]"
	}
	if props.VisualImpaired {
		flags += " [Visual Impaired]"
	}
	if props.Commentary {
		flags += " [Commentary]"
	}
	if props.OriginalLanguage {
		flags += " [Original]"
	}
	if props.TextDescriptions {
		flags += " [Text Descriptions]"
	}

	return flags
}

func (track *EbmlTrack) formatTrackInfo() string {
	if track == nil {
		return "None"
	}
	props := track.Properties
	flags := track.getFlags()
	return fmt.Sprintf("#%02d (ID: %d) Lang: %s, Name: '%s', Flags:%s", track.TypeOrder, track.Properties.Number, props.Language, props.Name, flags)
}

func isRelevantTrack(track EbmlTrack) bool {
	return track.Type == "audio" || track.Type == "subtitles"
}

func validateTrackBasics(track EbmlTrack) *checks.CheckResult {
	lang := track.Properties.Language
	tag := language.Make(lang)
	if config.IsCheckEnabled("matroska_language_tag") && tag == language.Und {
		shortWarning := "missing or invalid language tag"
		return &checks.CheckResult{
			Identifier:  "matroska_language_tag",
			Description: "Track has valid language tag",
			Passed:      false,
			Severity:    "warning",
			Tracks:      []checks.TrackCheckResult{trackToResult(&track, false, shortWarning)},
		}
	}
	if config.IsCheckEnabled("matroska_multi_lang") && tag == language.Make("mul") && track.Properties.Name == "" {
		shortWarning := "missing Name for 'mul' language"
		return &checks.CheckResult{
			Identifier:  "matroska_multi_lang",
			Description: "Multi-language track must have a Name",
			Passed:      false,
			Severity:    "warning",
			Tracks:      []checks.TrackCheckResult{trackToResult(&track, false, shortWarning)},
		}
	}
	return nil
}

func checkOriginalLanguageConsistency(track EbmlTrack, langHasOriginalFlag map[string]bool) *checks.CheckResult {
	lang := track.Properties.Language
	if langHasOriginalFlag[lang] && !track.Properties.OriginalLanguage {
		shortWarning := "missing Original flag"
		return &checks.CheckResult{
			Identifier:  "matroska_original_language",
			Description: "Consistency of Original language flag",
			Passed:      false,
			Severity:    "info",
			Tracks:      []checks.TrackCheckResult{trackToResult(&track, false, shortWarning)},
		}
	}
	return nil
}

func checkDuplicateTracks(track EbmlTrack, seenTracks map[string]bool) *checks.CheckResult {
	props := track.Properties
	trackKey := fmt.Sprintf("%s-%s-%t-%t-%t-%t-%t-%t-%s",
		track.Type, props.Language, props.Default, props.Forced,
		props.HearingImpaired, props.VisualImpaired,
		props.Commentary, props.OriginalLanguage, props.Name)
	if seenTracks[trackKey] {
		shortWarning := "duplicate track"
		return &checks.CheckResult{
			Identifier:  "matroska_duplicate_tracks",
			Description: "No duplicate tracks",
			Passed:      false,
			Severity:    "warning",
			Tracks:      []checks.TrackCheckResult{trackToResult(&track, false, shortWarning)},
		}
	}
	seenTracks[trackKey] = true
	return nil
}

func checkNameKeywords(track EbmlTrack) *checks.CheckResult {
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
			shortWarning := "'mul' but Name has <2 language names"
			return &checks.CheckResult{
				Identifier:  "matroska_name_keywords",
				Description: "Multi-language track contains language names in Name field",
				Passed:      false,
				Severity:    "warning",
				Tracks:      []checks.TrackCheckResult{trackToResult(&track, false, shortWarning)},
			}
		}
	}

	return nil
}

func checkFlagKeywordResult(track EbmlTrack, flag bool, flagName, keywordStr string, hasKeyword bool) *checks.CheckResult {
	if flag && !hasKeyword {
		shortWarning := fmt.Sprintf("is %s but Name missing '%s'", flagName, keywordStr)
		return &checks.CheckResult{
			Identifier:  "matroska_name_keywords",
			Description: "Track Name contains relevant flag keywords",
			Passed:      false,
			Severity:    "info",
			Tracks:      []checks.TrackCheckResult{trackToResult(&track, false, shortWarning)},
		}
	}
	if !flag && hasKeyword {
		shortWarning := fmt.Sprintf("'%s' in Name but no %s flag", keywordStr, flagName)
		return &checks.CheckResult{
			Identifier:  "matroska_name_keywords",
			Description: "Track Name contains relevant flag keywords",
			Passed:      false,
			Severity:    "info",
			Tracks:      []checks.TrackCheckResult{trackToResult(&track, false, shortWarning)},
		}
	}
	return nil
}
func getTrackPriority(track EbmlTrack) int {
	lang := track.Properties.Language
	tag := language.Make(lang)
	prefTag := language.Make(config.GetPreferredLanguage())

	// Language Score (1000s)
	// Preferred: 1000
	// Original (if flag_original is set): 2000
	// Multiple (mul): 3000
	// English (eng): 4000
	// Alphabetical: 5000 + (char sum or similar to keep relative order)

	langScore := 5000
	if tag == prefTag {
		langScore = 1000
	} else if track.Properties.OriginalLanguage {
		langScore = 2000
	} else if tag == language.Make("mul") {
		langScore = 3000
	} else if tag == language.English {
		langScore = 4000
	} else {
		// Basic alphabetical offset for the rest
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
		// Default (0) -> AD (10) -> Simple Language (20) -> Commentary (30)
		if track.Properties.Commentary {
			propertyScore = 30
		} else if track.Properties.VisualImpaired {
			propertyScore = 10
		} else if track.Properties.TextDescriptions {
			// Often used for Simple Language/Easy access if not AD
			propertyScore = 20
		}
	} else if track.Type == "subtitles" {
		// Forced (0) -> Default (10) -> SDH (20)
		if track.Properties.Forced {
			propertyScore = 0
		} else if track.Properties.HearingImpaired {
			propertyScore = 20
		} else {
			propertyScore = 10
		}
	}

	return langScore + propertyScore
}

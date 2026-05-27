package matroska

import (
	"fmt"
	"regexp"
	"strings"

	"codeberg.org/n0ne/parsec/internal/config"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

func VerifyTrackOrder(tracks []EbmlTrack) error {
	var lastAudio, lastSub *EbmlTrack
	var lastAudioPriority, lastSubPriority int
	seenTracks := make(map[string]bool)
	langHasOriginalFlag := make(map[string]bool)

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
			if err := validateTrackBasics(*track); err != nil {
				return err
			}
		}

		if config.IsCheckEnabled("matroska_name_quality") {
			if err := checkTrackNameQuality(*track); err != nil {
				fmt.Printf("QA Warning: %v\n", err)
			}
		}

		if config.IsCheckEnabled("matroska_name_codecs") {
			if err := checkTrackNameCodecs(*track); err != nil {
				fmt.Printf("QA Warning: %v\n", err)
			}
		}

		if config.IsCheckEnabled("matroska_name_redundant_lang") {
			if err := checkTrackNameRedundantLang(*track); err != nil {
				fmt.Printf("QA Warning: %v\n", err)
			}
		}

		if config.IsCheckEnabled("matroska_original_language") {
			if err := checkOriginalLanguageConsistency(*track, langHasOriginalFlag); err != nil {
				return err
			}
		}

		if config.IsCheckEnabled("matroska_duplicate_tracks") {
			if err := checkDuplicateTracks(*track, seenTracks); err != nil {
				return err
			}
		}

		if config.IsCheckEnabled("matroska_name_keywords") {
			if err := checkNameKeywords(*track); err != nil {
				return err
			}
		}

		if config.IsCheckEnabled("matroska_track_order") {
			priority := getTrackPriority(*track)
			if track.Type == "audio" {
				if priority < lastAudioPriority {
					return fmt.Errorf("audio track #%02d (ID: %d) is out of order.\n  Previous: %s\n  Current:  %s",
						track.TypeOrder, track.Properties.Number, formatTrackInfo(lastAudio), formatTrackInfo(track))
				}
				lastAudio = track
				lastAudioPriority = priority
			} else if track.Type == "subtitles" {
				if priority < lastSubPriority {
					return fmt.Errorf("subtitle track #%02d (ID: %d) is out of order.\n  Previous: %s\n  Current:  %s",
						track.TypeOrder, track.Properties.Number, formatTrackInfo(lastSub), formatTrackInfo(track))
				}
				lastSub = track
				lastSubPriority = priority
			}
		}
	}

	return nil
}

func checkTrackNameQuality(track EbmlTrack) error {
	junkKeywords := []string{"STEREO", "SURROUND", "EXTERNAL", "UPLOADED", "ENCODED"}
	nameUpper := strings.ToUpper(track.Properties.Name)
	for _, junk := range junkKeywords {
		if strings.Contains(nameUpper, junk) {
			return fmt.Errorf("%s track #%02d (ID: %d) has junk keyword '%s' in Name field: '%s'", track.Type, track.TypeOrder, track.Properties.Number, junk, track.Properties.Name)
		}
	}
	return nil
}

func checkTrackNameCodecs(track EbmlTrack) error {
	nameUpper := strings.ToUpper(track.Properties.Name)
	// Simple Codecs
	simpleCodecs := []string{"AC3", "AAC", "E-AC3", "EAC3", "FLAC"}
	for _, codec := range simpleCodecs {
		if strings.Contains(nameUpper, codec) {
			return fmt.Errorf("%s track #%02d (ID: %d) contains simple codec '%s' in Name field: '%s'", track.Type, track.TypeOrder, track.Properties.Number, codec, track.Properties.Name)
		}
	}
	// Special handling for DTS (allow DTS-HD, DTS:X etc)
	if strings.Contains(nameUpper, "DTS") && !strings.Contains(nameUpper, "DTS-HD") && !strings.Contains(nameUpper, "DTS:X") && !strings.Contains(nameUpper, "DTS-ES") {
		re := regexp.MustCompile(`\bDTS\b`)
		if re.MatchString(nameUpper) {
			return fmt.Errorf("%s track #%02d (ID: %d) contains simple codec 'DTS' in Name field: '%s'", track.Type, track.TypeOrder, track.Properties.Number, track.Properties.Name)
		}
	}
	return nil
}

func checkTrackNameRedundantLang(track EbmlTrack) error {
	// Redundant Language Name
	if isRedundantLanguageName(track.Properties.Name, track.Properties.Language) {
		return fmt.Errorf("%s track #%02d (ID: %d) has redundant language name in Name field: '%s'", track.Type, track.TypeOrder, track.Properties.Number, track.Properties.Name)
	}
	return nil
}

func isRedundantLanguageName(name, trackLang string) bool {
	if name == "" {
		return false
	}

	tag := language.Make(trackLang)
	base, _ := tag.Base()
	target := base.String()

	// Split by space and punctuation
	re := regexp.MustCompile(`[\s/.,;()]+`)
	words := re.Split(name, -1)

	for _, word := range words {
		if getLanguageCodeFromName(word) == target {
			return true
		}
	}
	return false
}

func countLanguagesInString(name string) int {
	if name == "" {
		return 0
	}

	re := regexp.MustCompile(`[\s/.,;()]+`)
	words := re.Split(name, -1)

	count := 0
	for _, word := range words {
		if isLanguageName(word) {
			count++
		}
	}
	return count
}

func isLanguageName(word string) bool {
	return getLanguageCodeFromName(word) != ""
}

func getLanguageCodeFromName(word string) string {
	if len(word) <= 3 {
		return ""
	}

	wordLower := strings.ToLower(word)

	commonNames := map[string]string{
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

	if base := commonNames[wordLower]; base != "" {
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
func CheckDefaultFlags(tracks []EbmlTrack) error {
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
			if shouldBeDefault {
				return fmt.Errorf("%s track #%02d (ID: %d lang: %s) should have the Default flag set (it is the first standard track for this language)", track.Type, track.TypeOrder, track.Properties.Number, props.Language)
			}
			if isSpecialized {
				return fmt.Errorf("%s track #%02d (ID: %d, lang: %s) should NOT have the Default flag set because it is a specialized track (Forced/AD/SDH/Commentary/Simple)", track.Type, track.TypeOrder, track.Properties.Number, props.Language)
			}
			return fmt.Errorf("%s track #%02d (ID: %d, lang: %s) should NOT have the Default flag set (only the first standard track per language should be default)", track.Type, track.TypeOrder, track.Properties.Number, props.Language)
		}
	}
	return nil
}

func CheckSubtitleFormat(tracks []EbmlTrack) error {
	for _, track := range tracks {
		codec := track.Codec
		if track.Type == "subtitles" && !strings.Contains(codec, "SRT") {
			return fmt.Errorf("%s track #%d (ID: %d, lang: %s) is not a SRT subtitle track", track.Type, track.TypeOrder, track.Properties.Number, track.Codec)
		}
	}
	return nil
}

func formatTrackInfo(track *EbmlTrack) string {
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
		flags += " [SDH]"
	}
	if props.VisualImpaired {
		flags += " [AD]"
	}
	if props.Commentary {
		flags += " [Commentary]"
	}
	if props.OriginalLanguage {
		flags += " [Original]"
	}
	if props.TextDescriptions {
		flags += " [Simple]"
	}

	return fmt.Sprintf("#%02d (ID: %d) Lang: %s, Name: '%s', Flags:%s", track.TypeOrder, track.Properties.Number, props.Language, props.Name, flags)
}

func isRelevantTrack(track EbmlTrack) bool {
	return track.Type == "audio" || track.Type == "subtitles"
}

func validateTrackBasics(track EbmlTrack) error {
	lang := track.Properties.Language
	tag := language.Make(lang)
	if config.IsCheckEnabled("matroska_language_tag") && tag == language.Und {
		return fmt.Errorf("%s track #%d (ID: %d) is missing a valid language tag (got: %s)", track.Type, track.TypeOrder, track.Properties.Number, lang)
	}
	if config.IsCheckEnabled("matroska_multi_lang") && tag == language.Make("mul") && track.Properties.Name == "" {
		return fmt.Errorf("%s track #%d (ID: %d) with language 'mul' must have a Name field", track.Type, track.TypeOrder, track.Properties.Number)
	}
	return nil
}

func checkOriginalLanguageConsistency(track EbmlTrack, langHasOriginalFlag map[string]bool) error {
	lang := track.Properties.Language
	if langHasOriginalFlag[lang] && !track.Properties.OriginalLanguage {
		return fmt.Errorf("%s track #%d (ID: %d, lang: %s) is missing the OriginalLanguage flag (other tracks in this language have it)", track.Type, track.TypeOrder, track.Properties.Number, lang)
	}
	return nil
}

func checkDuplicateTracks(track EbmlTrack, seenTracks map[string]bool) error {
	props := track.Properties
	trackKey := fmt.Sprintf("%s-%s-%t-%t-%t-%t-%t-%t-%s",
		track.Type, props.Language, props.Default, props.Forced,
		props.HearingImpaired, props.VisualImpaired,
		props.Commentary, props.OriginalLanguage, props.Name)
	if seenTracks[trackKey] {
		return fmt.Errorf("%s track #%d (ID: %d) is a duplicate of a previous track (same language, flags, and name)", track.Type, track.TypeOrder, track.Properties.Number)
	}
	seenTracks[trackKey] = true
	return nil
}

func checkNameKeywords(track EbmlTrack) error {
	props := track.Properties
	nameUpper := strings.ToUpper(props.Name)

	if props.HearingImpaired && !strings.Contains(nameUpper, "SDH") {
		return fmt.Errorf("%s track #%d (ID: %d) is hearing impaired but Name field does not contain 'SDH'", track.Type, track.TypeOrder, track.Properties.Number)
	}
	if !props.HearingImpaired && strings.Contains(nameUpper, "SDH") {
		return fmt.Errorf("%s track #%d (ID: %d) has 'SDH' in Name field but is not flagged as hearing impaired", track.Type, track.TypeOrder, track.Properties.Number)
	}

	if props.Forced && !strings.Contains(nameUpper, "FORCED") {
		return fmt.Errorf("%s track #%d (ID: %d) is forced but Name field does not contain 'Forced'", track.Type, track.TypeOrder, track.Properties.Number)
	}
	if !props.Forced && strings.Contains(nameUpper, "FORCED") {
		return fmt.Errorf("%s track #%d (ID: %d) has 'Forced' in Name field but is not flagged as forced", track.Type, track.TypeOrder, track.Properties.Number)
	}

	if props.Commentary && !strings.Contains(nameUpper, "COMMENTARY") {
		return fmt.Errorf("%s track #%d (ID: %d) is commentary but Name field does not contain 'Commentary'", track.Type, track.TypeOrder, track.Properties.Number)
	}
	if !props.Commentary && strings.Contains(nameUpper, "COMMENTARY") {
		return fmt.Errorf("%s track #%d (ID: %d) has 'Commentary' in Name field but is not flagged as commentary", track.Type, track.TypeOrder, track.Properties.Number)
	}

	reAD := regexp.MustCompile(`\bAD\b`)
	if props.VisualImpaired {
		if !strings.Contains(nameUpper, "DESCRIPTIVE") && !strings.Contains(nameUpper, "DESCRIPTION") && !reAD.MatchString(nameUpper) {
			return fmt.Errorf("%s track #%d (ID: %d) is visual impaired but Name field does not contain 'Descriptive', 'Description', or 'AD'", track.Type, track.TypeOrder, track.Properties.Number)
		}
	}
	if !props.VisualImpaired && (strings.Contains(nameUpper, "DESCRIPTIVE") || strings.Contains(nameUpper, "DESCRIPTION") || reAD.MatchString(nameUpper)) {
		return fmt.Errorf("%s track #%d (ID: %d) has visual impaired keywords in Name field but is not flagged as visual impaired", track.Type, track.TypeOrder, track.Properties.Number)
	}

	if props.Language == "mul" {
		if countLanguagesInString(props.Name) < 2 {
			return fmt.Errorf("%s track #%d (ID: %d) has language 'mul' but Name field does not contain at least two language names", track.Type, track.TypeOrder, track.Properties.Number)
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

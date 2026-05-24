package metadata

import (
	"fmt"
	"strings"
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

		if err := validateTrackBasics(*track); err != nil {
			return err
		}

		if err := checkOriginalLanguageConsistency(*track, langHasOriginalFlag); err != nil {
			return err
		}

		if err := checkDuplicateTracks(*track, seenTracks); err != nil {
			return err
		}

		if err := checkNameKeywords(*track); err != nil {
			return err
		}

		priority := getTrackPriority(*track)
		if track.Type == "audio" {
			if priority < lastAudioPriority {
				return fmt.Errorf("audio track %d is out of order.\n  Previous: %s\n  Current:  %s",
					track.ID, formatTrackInfo(lastAudio), formatTrackInfo(track))
			}
			lastAudio = track
			lastAudioPriority = priority
		} else if track.Type == "subtitles" {
			if priority < lastSubPriority {
				return fmt.Errorf("subtitle track %d is out of order.\n  Previous: %s\n  Current:  %s",
					track.ID, formatTrackInfo(lastSub), formatTrackInfo(track))
			}
			lastSub = track
			lastSubPriority = priority
		}
	}

	if err := checkDefaultFlags(tracks); err != nil {
		return err
	}

	return nil
}

func checkDefaultFlags(tracks []EbmlTrack) error {
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
				return fmt.Errorf("track %d (%s, %s) should have the Default flag set (it is the first standard track for this language)", track.ID, track.Type, props.Language)
			}
			if isSpecialized {
				return fmt.Errorf("track %d (%s, %s) should NOT have the Default flag set because it is a specialized track (Forced/AD/SDH/Commentary/Simple)", track.ID, track.Type, props.Language)
			}
			return fmt.Errorf("track %d (%s, %s) should NOT have the Default flag set (only the first standard track per language should be default)", track.ID, track.Type, props.Language)
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

	return fmt.Sprintf("ID: %d, Lang: %s, Name: '%s', Flags:%s", track.ID, props.Language, props.Name, flags)
}

func isRelevantTrack(track EbmlTrack) bool {
	return track.Type == "audio" || track.Type == "subtitles"
}

func validateTrackBasics(track EbmlTrack) error {
	lang := track.Properties.Language
	if lang == "" || lang == "und" {
		return fmt.Errorf("track %d (%s) is missing a valid language tag", track.ID, track.Type)
	}
	if lang == "mul" && track.Properties.Name == "" {
		return fmt.Errorf("track %d (%s) with language 'mul' must have a Name field", track.ID, track.Type)
	}
	return nil
}

func checkOriginalLanguageConsistency(track EbmlTrack, langHasOriginalFlag map[string]bool) error {
	lang := track.Properties.Language
	if langHasOriginalFlag[lang] && !track.Properties.OriginalLanguage {
		return fmt.Errorf("track %d (%s, %s) is missing the OriginalLanguage flag (other tracks in this language have it)", track.ID, track.Type, lang)
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
		return fmt.Errorf("track %d (%s) is a duplicate of a previous track (same language, flags, and name)", track.ID, track.Type)
	}
	seenTracks[trackKey] = true
	return nil
}

func checkNameKeywords(track EbmlTrack) error {
	props := track.Properties
	nameUpper := strings.ToUpper(props.Name)

	if props.HearingImpaired && !strings.Contains(nameUpper, "SDH") {
		return fmt.Errorf("track %d (%s) is hearing impaired but Name field does not contain 'SDH'", track.ID, track.Type)
	}
	if props.Forced && !strings.Contains(nameUpper, "FORCED") {
		return fmt.Errorf("track %d (%s) is forced but Name field does not contain 'Forced'", track.ID, track.Type)
	}
	if props.Commentary && !strings.Contains(nameUpper, "COMMENTARY") {
		return fmt.Errorf("track %d (%s) is commentary but Name field does not contain 'Commentary'", track.ID, track.Type)
	}
	if props.VisualImpaired && props.Name == "" {
		return fmt.Errorf("track %d (%s) is visual impaired but Name field is empty", track.ID, track.Type)
	}
	return nil
}

func getTrackPriority(track EbmlTrack) int {
	lang := track.Properties.Language

	// Language Score (1000s)
	// Preferred (ger): 1000
	// Original (if flag_original is set): 2000
	// Multiple (mul): 3000
	// English (eng): 4000
	// Alphabetical: 5000 + (char sum or similar to keep relative order)

	langScore := 5000
	if lang == "ger" {
		langScore = 1000
	} else if track.Properties.OriginalLanguage {
		langScore = 2000
	} else if lang == "mul" {
		langScore = 3000
	} else if lang == "eng" {
		langScore = 4000
	} else {
		// Basic alphabetical offset for the rest
		if len(lang) >= 3 {
			langScore += int(lang[0]-'a')*100 + int(lang[1]-'a')*10 + int(lang[2]-'a')
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

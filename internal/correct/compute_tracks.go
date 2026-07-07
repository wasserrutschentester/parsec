package correct

import (
	"regexp"
	"slices"
	"strings"

	"golang.org/x/text/language"

	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

// cleanSplitRegex tokenizes a track name for cleaning. Unlike wordSplitRegex it
// does not split on dots, so channel notations such as "5.1" stay intact.
var cleanSplitRegex = regexp.MustCompile(`[\s/,;()]+`)

// ComputeMatroskaFlagFixes inspects the tracks of a Matroska file and returns
// the track property edits required to satisfy the auto-fixable flag checks.
func ComputeMatroskaFlagFixes(tracks []matroska.EbmlTrack) []matroska.TrackEdit {
	builder := newFixBuilder()
	builder.computeDefaultFlagFixes(tracks)
	builder.computeOriginalFlagFixes(tracks)

	return builder.edits()
}

// ComputeMatroskaNameFixes inspects the tracks of a Matroska file and returns
// the track name edits required to satisfy the auto-fixable name-quality checks.
func ComputeMatroskaNameFixes(tracks []matroska.EbmlTrack) []matroska.TrackEdit {
	builder := newFixBuilder()
	builder.computeNameFixes(tracks)

	return builder.edits()
}

// fixBuilder accumulates property edits per track while preserving the order in
// which tracks are first touched, so the resulting edit list is deterministic.
type fixBuilder struct {
	order   []int
	byNum   map[int]map[string]string
	reasons map[int]map[string]string
}

func newFixBuilder() *fixBuilder {
	return &fixBuilder{
		byNum:   make(map[int]map[string]string),
		reasons: make(map[int]map[string]string),
	}
}

func (b *fixBuilder) set(number int, key, value, reason string) {
	props, ok := b.byNum[number]
	if !ok {
		props = make(map[string]string)
		b.byNum[number] = props
		b.reasons[number] = make(map[string]string)
		b.order = append(b.order, number)
	}

	props[key] = value
	if reason != "" {
		b.reasons[number][key] = reason
	}
}

func (b *fixBuilder) edits() []matroska.TrackEdit {
	edits := make([]matroska.TrackEdit, 0, len(b.order))
	for _, number := range b.order {
		edits = append(edits, matroska.TrackEdit{
			Number:  number,
			Props:   b.byNum[number],
			Reasons: b.reasons[number],
		})
	}

	return edits
}

//nolint:cyclop // Resolving default flags inherently requires several condition branches
func (b *fixBuilder) computeDefaultFlagFixes(tracks []matroska.EbmlTrack) {
	if !config.IsCheckEnabled(config.CheckMatroskaDefaultFlags) {
		return
	}

	audioCounts, subCounts := checks.GetTrackCounts(tracks)
	seenAudioLangs := make(map[string]bool)
	seenSubLangs := make(map[string]bool)

	for i := range tracks {
		track := tracks[i]
		if !checks.IsRelevantTrack(track) {
			continue
		}

		props := track.Properties
		isSpecialized := props.Forced || props.Commentary || props.VisualImpaired || props.HearingImpaired || props.TextDescriptions

		shouldBeDefault := false
		if !isSpecialized {
			shouldBeDefault = checks.DetermineShouldBeDefault(track, audioCounts, subCounts, seenAudioLangs, seenSubLangs)
		}

		if props.Default != shouldBeDefault {
			reason := "Resolve default flag conflicts"
			if shouldBeDefault {
				reason = "Set as default standard track"
			} else if isSpecialized {
				reason = "Strip default flag from specialized track"
			}

			b.set(props.Number, "flag-default", boolFlag(shouldBeDefault), reason)
		}
	}
}

func (b *fixBuilder) computeOriginalFlagFixes(tracks []matroska.EbmlTrack) {
	if !config.IsCheckEnabled(config.CheckMatroskaOriginalLanguage) {
		return
	}

	langHasOriginalFlag := checks.GetOriginalLanguageMap(tracks)

	for i := range tracks {
		track := tracks[i]
		if !checks.IsRelevantTrack(track) {
			continue
		}

		props := track.Properties
		if langHasOriginalFlag[props.Language] && !props.OriginalLanguage {
			b.set(props.Number, "flag-original", "1", "Infer original language from file context")
		}
	}
}

func (b *fixBuilder) computeNameFixes(tracks []matroska.EbmlTrack) {
	fixedNames := make(map[int]string, len(tracks))

	for i := range tracks {
		track := tracks[i]
		if !checks.IsRelevantTrack(track) {
			continue
		}

		if newName := fixedTrackName(track); newName != track.Properties.Name {
			// An empty value instructs SetTrackProperties to delete the name.
			b.set(track.Properties.Number, "name", newName, "Clean junk keywords or redundant language")
			fixedNames[track.Properties.Number] = newName
		} else {
			fixedNames[track.Properties.Number] = track.Properties.Name
		}
	}

	b.computeCommentaryPairingNameFixes(tracks, fixedNames)
}

// fixedTrackName returns the cleaned track name, removing junk keywords, simple
// codec names and redundant language names, then appending any keyword that a
// set flag requires. It returns the original name unchanged when nothing needs
// fixing, so cosmetic separator differences never trigger an edit.
func fixedTrackName(track matroska.EbmlTrack) string {
	original := track.Properties.Name
	if original == "" {
		return ""
	}

	kept, removed := cleanNameTokens(original, track.Properties.Language)

	result := strings.Join(kept, " ")
	if !removed {
		result = original
	}

	result, prefixed := maybePrefixCommentaryName(result, track.Properties)

	result, appended := maybeAppendKeywords(result, track.Properties)
	if !removed && !appended {
		if !prefixed {
			return original
		}
	}

	return strings.TrimSpace(result)
}

// cleanNameTokens splits a track name and drops the tokens flagged by the
// enabled name checks, reporting whether anything was removed.
func cleanNameTokens(name, lang string) (kept []string, removed bool) {
	removeJunk := config.IsCheckEnabled(config.CheckMatroskaNameQuality)
	removeCodecs := config.IsCheckEnabled(config.CheckMatroskaNameCodecs)
	removeLang := config.IsCheckEnabled(config.CheckMatroskaNameRedundantLang)

	for _, token := range cleanSplitRegex.Split(name, -1) {
		switch {
		case token == "":
			continue
		case removeJunk && slices.Contains(checks.JunkKeywords, strings.ToUpper(token)):
			removed = true
		case removeCodecs && isSimpleCodecToken(token):
			removed = true
		case removeLang && isLanguageMatch(token, lang):
			removed = true
		default:
			kept = append(kept, token)
		}
	}

	return kept, removed
}

// maybeAppendKeywords appends the keywords required by the track's flags when
// the name-keyword check is enabled, reporting whether the name changed.
func maybeAppendKeywords(name string, props matroska.EbmlTrackProperties) (string, bool) {
	if !config.IsCheckEnabled(config.CheckMatroskaNameKeywords) {
		return name, false
	}

	withKeywords := appendFlagKeywords(name, props)

	return withKeywords, withKeywords != name
}

func maybePrefixCommentaryName(name string, props matroska.EbmlTrackProperties) (string, bool) {
	if !config.IsCheckEnabled(config.CheckMatroskaCommentaryPrefix) || !props.Commentary || strings.TrimSpace(name) == "" {
		return name, false
	}

	if checks.CommentaryPrefixRegex.MatchString(name) {
		return name, false
	}

	prefixed := addCommentaryPrefix(name)

	return prefixed, prefixed != name
}

func addCommentaryPrefix(name string) string {
	prefix, core := splitCommentaryPrefixContext(name)
	core = strings.TrimSpace(core)

	// A core of just "commentary" carries no actual attribution (no name/role
	// to put after "by"), so prefixing it would fabricate the meaningless
	// "Commentary by Commentary". Leave the name as-is for a human to fix.
	if core == "" || strings.EqualFold(core, "commentary") {
		return name
	}

	return prefix + "Commentary by " + core
}

func splitCommentaryPrefixContext(name string) (prefix, core string) {
	if before, after, ok := strings.Cut(name, "/"); ok {
		return strings.TrimSpace(before) + " / ", strings.TrimSpace(after)
	}

	return "", strings.TrimSpace(name)
}

func (b *fixBuilder) computeCommentaryPairingNameFixes(tracks []matroska.EbmlTrack, fixedNames map[int]string) {
	if !config.IsCheckEnabled(config.CheckMatroskaCommentaryPairing) {
		return
	}

	audioName, ok := soleAudioCommentaryName(tracks, fixedNames)
	if !ok {
		return
	}

	for _, track := range tracks {
		if track.Type != "subtitles" || !track.Properties.Commentary {
			continue
		}

		current := fixedNames[track.Properties.Number]
		if checks.ExtractCommentaryCore(current) == checks.ExtractCommentaryCore(audioName) {
			continue
		}

		newName, _ := maybeAppendKeywords(audioName, track.Properties)
		newName, _ = maybePrefixCommentaryName(newName, track.Properties)
		b.set(track.Properties.Number, "name", newName, "Generate missing multi-language track name")
		fixedNames[track.Properties.Number] = newName
	}
}

func soleAudioCommentaryName(tracks []matroska.EbmlTrack, fixedNames map[int]string) (string, bool) {
	var names []string

	for _, track := range tracks {
		if track.Type != "audio" || !track.Properties.Commentary {
			continue
		}

		name := strings.TrimSpace(fixedNames[track.Properties.Number])
		if name != "" {
			names = append(names, name)
		}
	}

	if len(names) != 1 {
		return "", false
	}

	return names[0], true
}

func isSimpleCodecToken(token string) bool {
	upper := strings.ToUpper(token)
	if slices.Contains(checks.SimpleCodecs, upper) {
		return true
	}

	// Standalone "DTS" only; "DTS-HD", "DTS:X" and "DTS-ES" are single tokens
	// and are intentionally preserved.
	return upper == "DTS"
}

func isLanguageMatch(word, trackLang string) bool {
	tag := language.Make(trackLang)

	base, _ := tag.Base()

	return checks.GetLanguageCodeFromName(word) == base.String()
}

func appendFlagKeywords(name string, props matroska.EbmlTrackProperties) string {
	appendKeyword := func(keyword string) {
		name = strings.TrimSpace(name + " " + keyword)
	}

	if props.HearingImpaired && !strings.Contains(strings.ToUpper(name), "SDH") {
		appendKeyword("SDH")
	}

	if props.Forced && !strings.Contains(strings.ToUpper(name), "FORCED") {
		appendKeyword("Forced")
	}

	if props.Commentary && !strings.Contains(strings.ToUpper(name), "COMMENTARY") {
		appendKeyword("Commentary")
	}

	if props.VisualImpaired && !hasVisualImpairedKeyword(strings.ToUpper(name)) {
		appendKeyword("Descriptive")
	}

	return name
}

func hasVisualImpairedKeyword(upper string) bool {
	return strings.Contains(upper, "DESCRIPTIVE") || strings.Contains(upper, "DESCRIPTION") || checks.ADRegex.MatchString(upper)
}

func boolFlag(value bool) string {
	if value {
		return "1"
	}

	return "0"
}

// NeedsLanguageFix reports whether a track is missing a valid language tag and
// the corresponding check is enabled. The correct value is unknown, so the
// caller must obtain it from the user.
func NeedsLanguageFix(track matroska.EbmlTrack) bool {
	if !config.IsCheckEnabled(config.CheckMatroskaLanguageTag) || !checks.IsRelevantTrack(track) {
		return false
	}

	return language.Make(track.Properties.Language) == language.Und
}

// NeedsMultiLangName reports whether a multi-language ("mul") track needs a
// descriptive Name that the tool cannot derive, so the caller must obtain it
// from the user. It covers two enabled-check conditions: matroska_multi_lang (a
// 'mul' track must have a Name at all) and matroska_name_keywords (a 'mul'
// track's Name must list at least two languages).
func NeedsMultiLangName(track matroska.EbmlTrack) bool {
	if !checks.IsRelevantTrack(track) || language.Make(track.Properties.Language) != language.Make("mul") {
		return false
	}

	name := track.Properties.Name

	if config.IsCheckEnabled(config.CheckMatroskaMultiLang) && name == "" {
		return true
	}

	return config.IsCheckEnabled(config.CheckMatroskaNameKeywords) && checks.CountLanguagesInString(name) < 2
}

// KeywordFlagFix describes a track whose name contains a keyword whose matching
// flag is not set. The resolution (set the flag or drop the word) is ambiguous,
// so the caller must ask the user.
type KeywordFlagFix struct {
	// Property is the mkvpropedit flag to set, e.g. "flag-commentary".
	Property string
	// Keyword is the keyword found in the track name, e.g. "Commentary".
	Keyword string
}

// ReverseKeywordFlagFixes returns the keyword/flag mismatches where the track
// name advertises a property (SDH, Forced, Commentary, descriptive) whose flag
// is not actually set. It returns nil when the name-keywords check is disabled.
func ReverseKeywordFlagFixes(track matroska.EbmlTrack) []KeywordFlagFix {
	if !config.IsCheckEnabled(config.CheckMatroskaNameKeywords) || !checks.IsRelevantTrack(track) {
		return nil
	}

	props := track.Properties
	upper := strings.ToUpper(props.Name)

	keywordFlags := []struct {
		flagSet  bool
		inName   bool
		property string
		keyword  string
	}{
		{props.HearingImpaired, strings.Contains(upper, "SDH"), "flag-hearing-impaired", "SDH"},
		{props.Forced, strings.Contains(upper, "FORCED"), "flag-forced", "Forced"},
		{props.Commentary, strings.Contains(upper, "COMMENTARY"), "flag-commentary", "Commentary"},
		{props.VisualImpaired, hasVisualImpairedKeyword(upper), "flag-visual-impaired", "Descriptive"},
	}

	var fixes []KeywordFlagFix

	for _, kf := range keywordFlags {
		if !kf.flagSet && kf.inName {
			fixes = append(fixes, KeywordFlagFix{Property: kf.property, Keyword: kf.keyword})
		}
	}

	return fixes
}

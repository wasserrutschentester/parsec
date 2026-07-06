package correct

import (
	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/metadata/mediainfo"
)

// cleanSplitRegex tokenizes a track name for cleaning. Unlike wordSplitRegex it
// does not split on dots, so channel notations such as "5.1" stay intact.

// ComputeMatroskaFlagFixes inspects the tracks of a Matroska file and returns
// the track property edits required to satisfy the auto-fixable flag checks:
// default-flag and original-language assignment. Checks that are disabled in
// the configuration are skipped, mirroring RunMatroskaChecks. Kept separate
// from ComputeMatroskaNameFixes so the two can be previewed and confirmed
// independently.

// ComputeMatroskaNameFixes inspects the tracks of a Matroska file and returns
// the track name edits required to satisfy the auto-fixable name-quality
// checks: junk keyword, codec and redundant-language removal, plus missing
// keyword appension. Checks that are disabled in the configuration are
// skipped, mirroring RunMatroskaChecks.

// ComputeContainerFixes returns the segment-level ("info") property edits
// needed to satisfy the title-, writing-application-, and creation-time
// privacy checks. A matching property is cleared rather than rewritten with
// a guessed replacement, mirroring the conservative junk-removal approach
// used for track names. Checks that are disabled in the configuration are
// skipped.
func ComputeContainerFixes(ebml *matroska.EbmlMetadata, meta *metadata.Metadata) []ContainerPropertyEdit {
	var props []ContainerPropertyEdit

	if config.IsCheckEnabled(config.CheckMatroskaTitleHygiene) && checks.TitleHygieneNeedsFix(ebml.Container.Properties.Title, meta) {
		newTitle := ""
		if meta != nil && meta.Title != "" {
			newTitle = meta.Title
		}

		props = append(props, ContainerPropertyEdit{Key: "title", OldValue: ebml.Container.Properties.Title, NewValue: newTitle})
	}

	if config.IsCheckEnabled(config.CheckMatroskaAppHygiene) && checks.AppHygieneNeedsFix(ebml.Container.Properties.WritingApplication) {
		props = append(props, ContainerPropertyEdit{Key: "writing-application", OldValue: ebml.Container.Properties.WritingApplication, NewValue: ""})
	}

	if config.IsCheckEnabled(config.CheckMatroskaCreationTimePrivacy) && checks.ContainerCreationTimeNeedsFix(ebml) {
		// Creation time is usually handled separately, but we leave it here if it was here.
		// Wait, the old code had `props["date"] = ""` here!
		// Let's preserve that.
		props = append(props, ContainerPropertyEdit{Key: "date", OldValue: "set", NewValue: ""})
	}

	return props
}

// ChapterAlignmentFix describes the timestamp corrections needed to align
// chapters with video keyframes, satisfying matroska_chapters_keyframe_alignment.
// Times is the full, ordered list of chapter start times (ns) to write back:
// mkvpropedit rewrites chapters by document position, so already-aligned
// chapters pass their original time through unchanged alongside the snapped
// ones. Changed counts how many entries actually differ from the original.

// ComputeChapterKeyframeSnaps returns the chapter timestamp corrections that
// align each misaligned chapter with its nearest video keyframe. Returns a
// zero-value ChapterAlignmentFix (Changed == 0) when the check is disabled,
// the file has no chapters, the video keyframe index can't be read, or
// nothing needs to change.

// extractChapterAtoms returns the chapter atoms to align. Real mkvmerge -J
// output never populates ebml.Chapters[0].Editions (it only reports
// num_entries), so in production this always extracts via mkvextract; the
// ebml-provided path exists so tests can inject known atoms without a real
// mkvextract round trip.

// snapChaptersToKeyframes returns, in chapter order, each chapter's start
// time snapped to its nearest keyframe when misaligned, or unchanged when
// already aligned, plus how many entries were actually snapped.

// nearestKeyframe returns the keyframe timestamp closest to timeStart.
// keyframes must be non-empty.

// ComputeUnusedFontAttachments returns the font attachments that satisfy the
// matroska_unused_fonts check's removal criteria: not referenced by any
// subtitle track's Styles or inline tags. Returns nil when the check is
// disabled in the configuration. attachmentFonts and usedFonts are the
// caller's already-computed checks.GetAttachmentFonts/checks.ComputeAllUsedFonts
// results: both are pure functions of ebml (which is a single fixed snapshot
// for a whole correct run), so recomputing them per fix step would only
// reparse identical data after an unrelated mkvpropedit edit bumps the
// file's mtime and busts their disk cache.

// FontRename describes a font attachment filename correction needed to
// satisfy the matroska_font_filename_compliance check. InternalNames lists
// every font name embedded in the attachment (a font file can carry more
// than one face/name); NewName is derived from the first one since a
// filename can only hold one.

// ComputeFontRenames returns the font attachments whose filename should be
// renamed to match the font's internal name. Unused attachments (not
// referenced by any subtitle track's Styles or inline tags, the same check
// matroska_unused_fonts uses) are excluded first: renaming a font that's
// about to be flagged for removal is pointless, and it's exactly the unused
// case that tends to produce duplicate copies of the same font under
// different names. Returns nil when the check is disabled or no remaining
// attachment carries a usable internal name. See ComputeUnusedFontAttachments
// for why attachmentNames/attachmentFonts/usedFonts are passed in rather
// than recomputed here.

// excludeAttachments returns the attachments in all that aren't present in
// exclude, by ID.

// computeFontRenames is the pure font-rename policy, shared with tests so
// font extraction (and therefore real font files) is not required to verify
// it. Renames that would collide on the same target filename (e.g. several
// distinct attachments all embedding "Times New Roman") are disambiguated
// with a " (2)", " (3)", ... suffix rather than silently colliding.
// attachmentNames is used for the compliance check and the display-only
// InternalNames field; the rename target itself always comes from
// checks.ProposedFontFilename (attachmentFonts), the exact same priority
// (PostScript name, then full name, then family name) and cleanup the check
// shows as its own "Proposed Name" column, so the two can never disagree
// about what a font should be renamed to.

// disambiguateFontRenames appends a numbered suffix to any rename whose
// target filename collides with an earlier one in the list.

// suffixFontName inserts " (n)" before the extension, e.g. "Times New
// Roman.ttf" -> "Times New Roman (2).ttf".

// fontRenameTarget builds a filename from internalName, keeping oldName's
// extension. Used for naming a newly-attached font (attachmentNameForFont in
// fonts.go), which has no parsed AttachmentFontInfo yet to run through
// checks.ProposedFontFilename's PostScript/full-name/family-name priority -
// not for renaming an existing attachment, which must use
// checks.ProposedFontFilename so it can never drift from what the
// matroska_font_filename_compliance check proposes.

// fixBuilder accumulates property edits per track while preserving the order in
// which tracks are first touched, so the resulting edit list is deterministic.

// An empty value instructs SetTrackProperties to delete the name.

// fixedTrackName returns the cleaned track name, removing junk keywords, simple
// codec names and redundant language names, then appending any keyword that a
// set flag requires. It returns the original name unchanged when nothing needs
// fixing, so cosmetic separator differences never trigger an edit.

// cleanNameTokens splits a track name and drops the tokens flagged by the
// enabled name checks, reporting whether anything was removed.

// maybeAppendKeywords appends the keywords required by the track's flags when
// the name-keyword check is enabled, reporting whether the name changed.

// A core of just "commentary" carries no actual attribution (no name/role
// to put after "by"), so prefixing it would fabricate the meaningless
// "Commentary by Commentary". Leave the name as-is for a human to fix.

// Standalone "DTS" only; "DTS-HD", "DTS:X" and "DTS-ES" are single tokens
// and are intentionally preserved.

// NeedsLanguageFix reports whether a track is missing a valid language tag and
// the corresponding check is enabled. The correct value is unknown, so the
// caller must obtain it from the user.

// NeedsMultiLangName reports whether a multi-language ("mul") track needs a
// descriptive Name that the tool cannot derive, so the caller must obtain it
// from the user. It covers two enabled-check conditions: matroska_multi_lang (a
// 'mul' track must have a Name at all) and matroska_name_keywords (a 'mul'
// track's Name must list at least two languages).

// KeywordFlagFix describes a track whose name contains a keyword whose matching
// flag is not set. The resolution (set the flag or drop the word) is ambiguous,
// so the caller must ask the user.

// Property is the mkvpropedit flag to set, e.g. "flag-commentary".

// Keyword is the keyword found in the track name, e.g. "Commentary".

// ReverseKeywordFlagFixes returns the keyword/flag mismatches where the track
// name advertises a property (SDH, Forced, Commentary, descriptive) whose flag
// is not actually set. It returns nil when the name-keywords check is disabled.

// RemovalKind identifies why a track is proposed for removal.

// RemovalUnwantedAudioLang identifies non-preferred, non-original audio removal.

// RemovalEmptyTrack identifies an audio track carrying no channels.

// RemovalCandidate describes a track proposed for removal during a remux,
// together with a human-readable reason. Removals are destructive, so the
// caller is expected to confirm each candidate with the user.

// MatroskaRemuxPlan describes the lossless remux operations needed to satisfy
// the remux-only Matroska checks. Track IDs are mkvmerge track IDs (the "id"
// field), as required by mkvmerge.

// TrackOrder is the desired output order of track IDs. It is nil when the
// tracks are already correctly ordered.

// StripCompressionIDs lists track IDs whose container compression should be
// removed.

// RemovalCandidates lists tracks proposed for removal (requires confirmation).

// IsEmpty reports whether the plan contains no work.

// ComputeMatroskaRemux inspects the tracks of a Matroska file and returns the
// remux operations needed to satisfy the checks that cannot be fixed in place:
// track ordering, container compression and removal of empty or unwanted-language
// audio tracks. originalLang is the MDB original language (may be
// empty); without it, unwanted-language pruning is skipped so the original
// track is never proposed for removal. Disabled checks are skipped.

// computeTrackOrder returns the desired output order (by track ID), grouping
// tracks as video, audio, subtitles and other while sorting audio and subtitle
// tracks by priority. It returns nil when the current order is already correct.

// 0 = zlib

// removalCollector accumulates removal candidates, keeping the first reason
// recorded for a track so the more specific signal (e.g. exact duplicate) wins.

// collectEmptyAudioTracks proposes removal of audio tracks reporting zero
// channels, satisfying mediainfo_empty_tracks for the part derivable from the
// EBML track properties alone (mkvmerge always reports audio_channels for a
// genuine audio track).

// Without the original language we cannot tell which non-preferred track is
// the legitimate original, so we skip pruning entirely to stay safe.

// ComputeMissingStatistics determines if a file needs statistics tags rebuilt.
func ComputeMissingStatistics(mi *mediainfo.MediaInfo) bool {
	if !config.IsCheckEnabled(config.CheckMediainfoMissingStatistics) {
		return false
	}

	return checks.MissingStatisticsNeedsFix(mi)
}

// ComputeCreationTimeTags determines if a file has creation time tags that need removal.
func ComputeCreationTimeTags(tagsXML []byte) bool {
	if !config.IsCheckEnabled(config.CheckMatroskaCreationTimePrivacy) {
		return false
	}

	_, removed := checks.StripCreationTimeTags(tagsXML)

	return len(removed) > 0
}

package correct

import (
	"fmt"
	"strings"

	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
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

// Creation time is usually handled separately, but we leave it here if it was here.
// Wait, the old code had `props["date"] = ""` here!
// Let's preserve that.

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
func ComputeUnusedFontAttachments(ebml *matroska.EbmlMetadata, attachmentFonts []matroska.AttachmentFontInfo, usedFonts map[checks.FontStyle]bool) []matroska.EbmlAttachment {
	if !config.IsCheckEnabled(config.CheckMatroskaUnusedFonts) {
		return nil
	}

	return checks.UnusedFontAttachments(ebml.Attachments, attachmentFonts, usedFonts)
}

// FontRename describes a font attachment filename correction needed to
// satisfy the matroska_font_filename_compliance check. InternalNames lists
// every font name embedded in the attachment (a font file can carry more
// than one face/name); NewName is derived from the first one since a
// filename can only hold one.
type FontRename struct {
	ID            int
	OldName       string
	NewName       string
	InternalNames []string
}

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
func ComputeFontRenames(ebml *matroska.EbmlMetadata, attachmentNames map[int][]string, attachmentFonts []matroska.AttachmentFontInfo, usedFonts map[checks.FontStyle]bool) []FontRename {
	if !config.IsCheckEnabled(config.CheckMatroskaFontFilenameCompliance) {
		return nil
	}

	unused := checks.UnusedFontAttachments(ebml.Attachments, attachmentFonts, usedFonts)

	return computeFontRenames(excludeAttachments(ebml.Attachments, unused), attachmentNames, attachmentFonts)
}

// excludeAttachments returns the attachments in all that aren't present in
// exclude, by ID.
func excludeAttachments(all, exclude []matroska.EbmlAttachment) []matroska.EbmlAttachment {
	excludedIDs := make(map[int]bool, len(exclude))
	for _, att := range exclude {
		excludedIDs[att.ID] = true
	}

	kept := make([]matroska.EbmlAttachment, 0, len(all))

	for _, att := range all {
		if !excludedIDs[att.ID] {
			kept = append(kept, att)
		}
	}

	return kept
}

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
func computeFontRenames(attachments []matroska.EbmlAttachment, attachmentNames map[int][]string, attachmentFonts []matroska.AttachmentFontInfo) []FontRename {
	var renames []FontRename

	for _, att := range attachments {
		if !checks.IsFontAttachment(att) {
			continue
		}

		names := attachmentNames[att.ID]
		if len(names) == 0 || checks.FontFilenameCompliant(att.FileName, names) {
			continue
		}

		newName := checks.ProposedFontFilename(att.FileName, att.ID, attachmentFonts)
		if newName == "" {
			continue
		}

		renames = append(renames, FontRename{
			ID:            att.ID,
			OldName:       att.FileName,
			NewName:       newName,
			InternalNames: names,
		})
	}

	return disambiguateFontRenames(renames)
}

// disambiguateFontRenames appends a numbered suffix to any rename whose
// target filename collides with an earlier one in the list.
func disambiguateFontRenames(renames []FontRename) []FontRename {
	seen := make(map[string]int, len(renames))

	for i, r := range renames {
		seen[r.NewName]++
		if n := seen[r.NewName]; n > 1 {
			renames[i].NewName = suffixFontName(r.NewName, n)
		}
	}

	return renames
}

// suffixFontName inserts " (n)" before the extension, e.g. "Times New
// Roman.ttf" -> "Times New Roman (2).ttf".
func suffixFontName(name string, n int) string {
	base, ext := name, ""
	if idx := strings.LastIndex(name, "."); idx != -1 {
		base, ext = name[:idx], name[idx:]
	}

	return fmt.Sprintf("%s (%d)%s", base, n, ext)
}

// fontRenameTarget builds a filename from internalName, keeping oldName's
// extension. Used for naming a newly-attached font (attachmentNameForFont in
// fonts.go), which has no parsed AttachmentFontInfo yet to run through
// checks.ProposedFontFilename's PostScript/full-name/family-name priority -
// not for renaming an existing attachment, which must use
// checks.ProposedFontFilename so it can never drift from what the
// matroska_font_filename_compliance check proposes.
func fontRenameTarget(oldName, internalName string) string {
	ext := ""
	if idx := strings.LastIndex(oldName, "."); idx != -1 {
		ext = oldName[idx:]
	}

	return internalName + ext
}

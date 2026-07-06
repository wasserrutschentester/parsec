package correct

import (
	"fmt"
	"strings"

	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

// ComputeUnusedFontAttachments returns the font attachments that satisfy the
// matroska_unused_fonts check's removal criteria.
// It relies on pre-computed attachmentFonts and usedFonts for performance.
func ComputeUnusedFontAttachments(ebml *matroska.EbmlMetadata, attachmentFonts []matroska.AttachmentFontInfo, usedFonts map[checks.FontStyle]bool) []matroska.EbmlAttachment {
	if !config.IsCheckEnabled(config.CheckMatroskaUnusedFonts) {
		return nil
	}

	return checks.UnusedFontAttachments(ebml.Attachments, attachmentFonts, usedFonts)
}

// FontRename describes a font attachment filename correction.
// InternalNames lists embedded fonts; NewName is derived from the first.
type FontRename struct {
	ID            int
	OldName       string
	NewName       string
	InternalNames []string
}

// ComputeFontRenames returns the font attachments whose filename should be
// renamed to match the font's internal name. Unused attachments are excluded.
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

// computeFontRenames is the pure font-rename policy.
// Renames that collide on target filename are disambiguated with a suffix.
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

// fontRenameTarget builds a filename from internalName, keeping oldName's extension.
// Used for naming a newly-attached font, which has no parsed AttachmentFontInfo.
func fontRenameTarget(oldName, internalName string) string {
	ext := ""
	if idx := strings.LastIndex(oldName, "."); idx != -1 {
		ext = oldName[idx:]
	}

	return internalName + ext
}

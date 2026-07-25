package correct

import (
	"strings"

	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

// ComputeUnusedFontAttachments returns the font attachments that satisfy the
// matroska_unused_fonts check's removal criteria.
// It relies on pre-computed attachmentFonts and usedFonts for performance.
func ComputeUnusedFontAttachments(ebml *matroska.EbmlMetadata, attachmentFonts []matroska.AttachmentFontInfo, usedFonts map[checks.FontStyle]bool) []checks.UnusedFont {
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
// renamed to match the font's internal name.
func ComputeFontRenames(ebml *matroska.EbmlMetadata, attachmentFonts []matroska.AttachmentFontInfo) []FontRename {
	if !config.IsCheckEnabled(config.CheckMatroskaFontFilenameCompliance) {
		return nil
	}

	proposed := checks.ComputeProposedFontRenames(ebml.Attachments, attachmentFonts)

	var renames []FontRename

	for _, p := range proposed {
		if p.ProposedName == "-" {
			continue
		}

		renames = append(renames, FontRename{
			ID:            p.AttachmentID,
			OldName:       p.CurrentName,
			NewName:       p.ProposedName,
			InternalNames: p.InternalNames,
		})
	}

	return renames
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

package correct

import (
	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/metadata/resolve"
)

// PlanFile analyzes a Matroska file and returns a complete FixPlan containing
// all proposed modifications without applying any changes.
func PlanFile(filePath string, opts Options) (*FixPlan, error) {
	plan := NewFixPlan()

	ebml, err := matroska.GetEbmlMetadata(filePath)
	if err != nil {
		// If we can't parse EBML, we can't plan any fixes
		return plan, nil
	}

	res, _ := resolve.Metadata(resolve.Options{
		FilePath: filePath,
		ImdbID:   opts.ImdbID,
		TmdbID:   opts.TmdbID,
		TvdbID:   opts.TvdbID,
	})
	meta := res.Meta

	// 1. Container Properties
	plan.ContainerProperties = ComputeContainerFixes(ebml, meta)

	computeFontsForPlan(plan, filePath, ebml)

	// 6. Missing Statistics
	plan.WriteStatistics = ComputeMissingStatistics(res.MediaInfo)

	// 7. Creation Time Tags
	tagsXML, _ := matroska.ExtractTagsXML(filePath)
	plan.ClearCreationTime = ComputeCreationTimeTags(tagsXML)

	// 8. Track Edits
	// For now we just append the basic flag and name fixes
	plan.TrackEdits = append(plan.TrackEdits, ComputeMatroskaNameFixes(ebml.Tracks)...)
	plan.TrackEdits = append(plan.TrackEdits, ComputeMatroskaFlagFixes(ebml.Tracks)...)

	// 9. Remux Plan
	remuxPlan := ComputeMatroskaRemux(ebml.Tracks, "en") // assuming English for now or lookupOriginalLanguage
	plan.RemuxRequired = len(remuxPlan.TrackOrder) > 0 || len(remuxPlan.RemovalCandidates) > 0 || len(remuxPlan.StripCompressionIDs) > 0

	plan.RemuxTrackOrder = remuxPlan.TrackOrder
	for _, candidate := range remuxPlan.RemovalCandidates {
		plan.RemuxRemoveTracks = append(plan.RemuxRemoveTracks, candidate.TrackID)
	}

	plan.RemuxStripCompression = remuxPlan.StripCompressionIDs

	return plan, nil
}

func computeFontsForPlan(plan *FixPlan, filePath string, ebml *matroska.EbmlMetadata) {
	attachmentFonts := checks.GetAttachmentFonts(filePath, ebml.Attachments)
	_, attachmentNames := FontMappingFromFonts(attachmentFonts)
	extracted := checks.ExtractSubtitleTracks(filePath, ebml.Tracks, true, false)
	usedFonts := checks.ComputeAllUsedFonts(ebml.Tracks, extracted)

	renames := ComputeFontRenames(ebml, attachmentNames, attachmentFonts, usedFonts)
	for _, r := range renames {
		plan.AttachmentRenames[r.ID] = r.NewName
	}

	plan.ChapterKeyframeSnaps = ComputeChapterKeyframeSnaps(filePath, ebml)

	missingPlan := ComputeMissingFontAttachments(filePath, ebml, attachmentFonts, false)
	for _, att := range missingPlan.Attachments {
		plan.FontsToAdd = append(plan.FontsToAdd, matroska.AttachmentAdd{
			Path:     att.Path,
			Name:     att.AttachmentName,
			MIMEType: att.MIMEType,
		})
	}

	unused := ComputeUnusedFontAttachments(ebml, attachmentFonts, usedFonts)
	for _, att := range unused {
		plan.FontsToRemove = append(plan.FontsToRemove, att.ID)
	}
}

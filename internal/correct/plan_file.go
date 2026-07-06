package correct

import (
	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/metadata/resolve"
)

var (
	resolveMetadata = resolve.Metadata
	getEbmlMetadata = matroska.GetEbmlMetadata
	extractTagsXML  = matroska.ExtractTagsXML
)

// PlanFile analyzes a Matroska file and returns a complete FixPlan containing
// all proposed modifications without applying any changes.
func PlanFile(filePath string, opts Options) (*FixPlan, error) {
	plan := NewFixPlan()

	ebml, err := getEbmlMetadata(filePath)
	if err != nil {
		// If we can't parse EBML, we can't plan any fixes
		return plan, nil
	}

	res, _ := resolveMetadata(resolve.Options{
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
	tagsXML, _ := extractTagsXML(filePath)
	plan.ClearCreationTime = ComputeCreationTimeTags(tagsXML)

	// 8. Track Edits
	// For now we just append the basic flag and name fixes
	plan.FlagEdits = ComputeMatroskaFlagFixes(ebml.Tracks)
	plan.NameEdits = ComputeMatroskaNameFixes(ebml.Tracks)

	// Apply automatic track edits to an in-memory copy before computing the remux plan
	// because track reordering depends on the *new* track languages!
	simulatedTracks := ApplyEditsToMemoryTracks(ebml.Tracks, plan.FlagEdits, plan.NameEdits)

	// 9. Remux Plan
	originalLang := lookupOriginalLanguage(filePath, ebml.Tracks, opts)
	remuxPlan := ComputeMatroskaRemux(simulatedTracks, originalLang)
	plan.RemuxRequired = len(remuxPlan.TrackOrder) > 0 || len(remuxPlan.RemovalCandidates) > 0 || len(remuxPlan.StripCompressionIDs) > 0

	plan.RemuxTrackOrder = remuxPlan.TrackOrder
	plan.RemuxRemoveTracks = append(plan.RemuxRemoveTracks, remuxPlan.RemovalCandidates...)
	plan.RemuxStripCompression = remuxPlan.StripCompressionIDs

	return plan, nil
}

// ApplyEditsToMemoryTracks creates a copy of the given tracks and applies the specified track edits to them.
func ApplyEditsToMemoryTracks(original []matroska.EbmlTrack, editGroups ...[]matroska.TrackEdit) []matroska.EbmlTrack {
	tracks := make([]matroska.EbmlTrack, len(original))
	copy(tracks, original)

	for _, group := range editGroups {
		for _, edit := range group {
			for i := range tracks {
				if tracks[i].Properties.Number == edit.Number {
					for k, v := range edit.Props {
						applyPropertyToTrack(&tracks[i], k, v)
					}
				}
			}
		}
	}

	return tracks
}

func applyPropertyToTrack(track *matroska.EbmlTrack, key, value string) {
	switch key {
	case "language":
		track.Properties.Language = value
	case "name":
		track.Properties.Name = value
	case "flag-default":
		track.Properties.Default = value == "1"
	case "flag-forced":
		track.Properties.Forced = value == "1"
	case "flag-hearing-impaired":
		track.Properties.HearingImpaired = value == "1"
	case "flag-visual-impaired":
		track.Properties.VisualImpaired = value == "1"
	case "flag-original":
		track.Properties.OriginalLanguage = value == "1"
	case "flag-commentary":
		track.Properties.Commentary = value == "1"
	}
}

func computeFontsForPlan(plan *FixPlan, filePath string, ebml *matroska.EbmlMetadata) {
	attachmentFonts := checks.GetAttachmentFonts(filePath, ebml.Attachments)
	_, attachmentNames := FontMappingFromFonts(attachmentFonts)
	extracted := checks.ExtractSubtitleTracks(filePath, ebml.Tracks, true, false)
	usedFonts := checks.ComputeAllUsedFonts(ebml.Tracks, extracted)

	renames := ComputeFontRenames(ebml, attachmentNames, attachmentFonts, usedFonts)
	for _, r := range renames {
		plan.AttachmentRenames = append(plan.AttachmentRenames, AttachmentRename{
			ID:      r.ID,
			OldName: r.OldName,
			NewName: r.NewName,
		})
	}

	plan.ChapterKeyframeSnaps = ComputeChapterKeyframeSnaps(filePath, ebml)

	missingPlan := ComputeMissingFontAttachments(filePath, ebml, attachmentFonts, false)
	for _, att := range missingPlan.Attachments {
		plan.FontsToAdd = append(plan.FontsToAdd, MissingFontAttachment{
			Path:           att.Path,
			AttachmentName: att.AttachmentName,
			MIMEType:       att.MIMEType,
			FontName:       att.FontName,
			Source:         att.Source,
		})
	}

	unused := ComputeUnusedFontAttachments(ebml, attachmentFonts, usedFonts)
	for _, att := range unused {
		plan.FontsToRemove = append(plan.FontsToRemove, AttachmentRemove{
			ID:   att.ID,
			Name: att.FileName,
		})
	}
}

package correct

import (
	"maps"
	"strings"

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
	plan.Metadata.Container.Properties = ComputeContainerFixes(ebml, meta)

	computeFontsForPlan(plan, filePath, ebml)

	// 6. Missing Statistics
	plan.Metadata.Container.WriteStatistics = ComputeMissingStatistics(res.MediaInfo)

	// 7. Creation Time Tags
	tagsXML, _ := extractTagsXML(filePath)
	plan.Metadata.Container.ClearCreationTime = ComputeCreationTimeTags(tagsXML)

	// 8. Track Edits
	// For now we just append the basic flag and name fixes
	flagEdits := ComputeMatroskaFlagFixes(ebml.Tracks)
	nameEdits := ComputeMatroskaNameFixes(ebml.Tracks)

	plan.Metadata.Tracks = mergeTrackEdits(flagEdits, nameEdits)

	// Apply automatic track edits to an in-memory copy before computing the remux plan
	// because track reordering depends on the *new* track languages!
	simulatedTracks := ApplyEditsToMemoryTracks(ebml.Tracks, plan.Metadata.Tracks)

	// 9. Remux Plan
	originalLang := lookupOriginalLanguage(filePath, ebml.Tracks, opts)
	remuxPlan := ComputeMatroskaRemux(simulatedTracks, originalLang)
	plan.Remux.Required = len(remuxPlan.TrackOrder) > 0 || len(remuxPlan.RemovalCandidates) > 0 || len(remuxPlan.StripCompressionIDs) > 0

	plan.Remux.TrackOrder = remuxPlan.TrackOrder
	plan.Remux.RemoveTracks = append(plan.Remux.RemoveTracks, remuxPlan.RemovalCandidates...)
	plan.Remux.StripCompression = remuxPlan.StripCompressionIDs

	return plan, nil
}

func mergeTrackEdits(editGroups ...[]matroska.TrackEdit) []matroska.TrackEdit {
	merged := make(map[int]*matroska.TrackEdit)

	var order []int

	for _, group := range editGroups {
		for _, e := range group {
			if _, ok := merged[e.Number]; !ok {
				order = append(order, e.Number)
				merged[e.Number] = &matroska.TrackEdit{
					Number:  e.Number,
					Props:   make(map[string]string),
					Reasons: make(map[string]string),
				}
			}

			maps.Copy(merged[e.Number].Props, e.Props)

			for k, v := range e.Reasons {
				if merged[e.Number].Reasons == nil {
					merged[e.Number].Reasons = make(map[string]string)
				}

				merged[e.Number].Reasons[k] = v
			}
		}
	}

	res := make([]matroska.TrackEdit, 0, len(order))
	for _, num := range order {
		res = append(res, *merged[num])
	}

	return res
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

//nolint:cyclop,funlen // loop over nested slices adds complexity but is readable
func computeFontsForPlan(plan *FixPlan, filePath string, ebml *matroska.EbmlMetadata) {
	attachmentFonts := checks.GetAttachmentFonts(filePath, ebml.Attachments)
	extracted := checks.ExtractSubtitleTracks(filePath, ebml.Tracks, true, false)
	usedFonts := checks.ComputeAllUsedFonts(ebml.Tracks, extracted)

	renames := ComputeFontRenames(ebml, attachmentFonts)
	for _, r := range renames {
		fullName := "-"
		psName := "-"

		for _, fInfo := range attachmentFonts {
			if fInfo.AttachmentID == r.ID {
				if len(fInfo.FullNames) > 0 {
					fullName = strings.Join(fInfo.FullNames, ", ")
				}

				if fInfo.PostScriptName != "" {
					psName = fInfo.PostScriptName
				}

				break
			}
		}

		plan.Metadata.Attachments.Renames = append(plan.Metadata.Attachments.Renames, AttachmentRename{
			ID:             r.ID,
			OldName:        r.OldName,
			NewName:        r.NewName,
			FullName:       fullName,
			PostScriptName: psName,
		})
	}

	plan.Metadata.Chapters.KeyframeSnaps = ComputeChapterKeyframeSnaps(filePath, ebml)

	missingPlan := ComputeMissingFontAttachments(filePath, ebml, attachmentFonts, false)
	for _, att := range missingPlan.Attachments {
		plan.Metadata.Attachments.ToAdd = append(plan.Metadata.Attachments.ToAdd, MissingFontAttachment{
			Path:           att.Path,
			AttachmentName: att.AttachmentName,
			MIMEType:       att.MIMEType,
			FontName:       att.FontName,
			Source:         att.Source,
			RequestedBy:    att.RequestedBy,
		})
	}

	unused := ComputeUnusedFontAttachments(ebml, attachmentFonts, usedFonts)
	for _, u := range unused {
		att := u.Attachment
		fullName := "-"

		for _, fInfo := range attachmentFonts {
			if fInfo.AttachmentID == att.ID {
				if len(fInfo.FullNames) > 0 {
					fullName = strings.Join(fInfo.FullNames, ", ")
				} else if fInfo.FamilyName != "" {
					fullName = fInfo.FamilyName
				}

				break
			}
		}

		plan.Metadata.Attachments.ToRemove = append(plan.Metadata.Attachments.ToRemove, AttachmentRemove{
			ID:        att.ID,
			Name:      att.FileName,
			FullName:  fullName,
			SizeBytes: int64(att.Size),
			Reason:    u.Reason,
		})
	}
}

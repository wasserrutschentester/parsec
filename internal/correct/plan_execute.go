package correct

import (
	"fmt"

	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/ui"
)

var (
	execSetContainerProperties = matroska.SetContainerProperties
	execRenameAttachments      = matroska.RenameAttachments
	execAddAttachments         = matroska.AddAttachments
	execDeleteAttachments      = matroska.DeleteAttachments
	execRewriteChapterTimes    = matroska.RewriteChapterTimestamps
	execAddTrackStatistics     = matroska.AddTrackStatisticsTags
	execExtractTagsXML         = matroska.ExtractTagsXML
	execSetTagsXML             = matroska.SetTagsXML
	execSetTrackProperties     = matroska.SetTrackProperties
	execRemuxTracks            = matroska.RemuxTracks
)

// ExecutePlan applies the non-destructive and destructive edits contained within
// a FixPlan to a Matroska file.
func ExecutePlan(filePath string, plan *FixPlan) error {
	if err := executeContainerProperties(filePath, plan); err != nil {
		return err
	}

	if err := executeAttachments(filePath, plan); err != nil {
		return err
	}

	if err := executeChapters(filePath, plan); err != nil {
		return err
	}

	if err := executeTracksAndTags(filePath, plan); err != nil {
		return err
	}

	if err := executeRemux(filePath, plan); err != nil {
		return err
	}

	return nil
}

func executeContainerProperties(filePath string, plan *FixPlan) error {
	if len(plan.ContainerProperties) > 0 {
		props := make(map[string]string)
		for _, p := range plan.ContainerProperties {
			props[p.Key] = p.NewValue
		}

		if err := execSetContainerProperties(filePath, props); err != nil {
			return fmt.Errorf("setting container properties: %w", err)
		}
	}

	return nil
}

func executeAttachments(filePath string, plan *FixPlan) error {
	if len(plan.AttachmentRenames) > 0 {
		renames := make(map[int]string)
		for _, r := range plan.AttachmentRenames {
			renames[r.ID] = r.NewName
		}

		if err := execRenameAttachments(filePath, renames); err != nil {
			return fmt.Errorf("renaming attachments: %w", err)
		}
	}

	if len(plan.FontsToAdd) > 0 {
		var adds []matroska.AttachmentAdd
		for _, f := range plan.FontsToAdd {
			adds = append(adds, matroska.AttachmentAdd{
				Path:     f.Path,
				Name:     f.AttachmentName,
				MIMEType: f.MIMEType,
			})
		}

		if err := execAddAttachments(filePath, adds); err != nil {
			return fmt.Errorf("adding font attachments: %w", err)
		}
	}

	if len(plan.FontsToRemove) > 0 {
		var removes []int
		for _, f := range plan.FontsToRemove {
			removes = append(removes, f.ID)
		}

		if err := execDeleteAttachments(filePath, removes); err != nil {
			return fmt.Errorf("removing font attachments: %w", err)
		}
	}

	return nil
}

func executeChapters(filePath string, plan *FixPlan) error {
	if plan.ChapterKeyframeSnaps.Changed > 0 && len(plan.ChapterKeyframeSnaps.Times) > 0 {
		if err := execRewriteChapterTimes(filePath, plan.ChapterKeyframeSnaps.Times); err != nil {
			return fmt.Errorf("rewriting chapter timestamps: %w", err)
		}
	}

	return nil
}

func executeTracksAndTags(filePath string, plan *FixPlan) error {
	if plan.WriteStatistics {
		if err := execAddTrackStatistics(filePath); err != nil {
			return fmt.Errorf("adding track statistics tags: %w", err)
		}
	}

	if plan.ClearCreationTime {
		tagsXML, err := execExtractTagsXML(filePath)
		if err != nil {
			return fmt.Errorf("extracting tags xml: %w", err)
		}

		stripped, removed := checks.StripCreationTimeTags(tagsXML)
		if len(removed) > 0 {
			if err := execSetTagsXML(filePath, stripped); err != nil {
				return fmt.Errorf("setting tags xml: %w", err)
			}
		}
	}

	allTrackEdits := make([]matroska.TrackEdit, 0, len(plan.FlagEdits)+len(plan.NameEdits)+len(plan.LanguageEdits))
	allTrackEdits = append(allTrackEdits, plan.FlagEdits...)
	allTrackEdits = append(allTrackEdits, plan.NameEdits...)
	allTrackEdits = append(allTrackEdits, plan.LanguageEdits...)

	if len(allTrackEdits) > 0 {
		if err := execSetTrackProperties(filePath, allTrackEdits); err != nil {
			return fmt.Errorf("setting track properties: %w", err)
		}
	}

	return nil
}

func executeRemux(filePath string, plan *FixPlan) error {
	if !plan.RemuxRequired {
		return nil
	}

	var removeIDs []int
	for _, r := range plan.RemuxRemoveTracks {
		removeIDs = append(removeIDs, r.TrackID)
	}

	remuxOpts := matroska.RemuxOptions{
		TrackOrder:          plan.RemuxTrackOrder,
		StripCompressionIDs: plan.RemuxStripCompression,
		RemoveTrackIDs:      removeIDs,
	}

	ui.Println(ui.Muted.Render("Remuxing... this may take a while for large files."))

	if err := execRemuxTracks(filePath, remuxOpts); err != nil {
		return fmt.Errorf("remuxing tracks: %w", err)
	}

	return nil
}

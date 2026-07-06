package correct

import (
	"fmt"

	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/ui"
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
		if err := matroska.SetContainerProperties(filePath, plan.ContainerProperties); err != nil {
			return fmt.Errorf("setting container properties: %w", err)
		}
	}

	return nil
}

func executeAttachments(filePath string, plan *FixPlan) error {
	if len(plan.AttachmentRenames) > 0 {
		if err := matroska.RenameAttachments(filePath, plan.AttachmentRenames); err != nil {
			return fmt.Errorf("renaming attachments: %w", err)
		}
	}

	if len(plan.FontsToAdd) > 0 {
		if err := matroska.AddAttachments(filePath, plan.FontsToAdd); err != nil {
			return fmt.Errorf("adding attachments: %w", err)
		}
	}

	if len(plan.FontsToRemove) > 0 {
		if err := matroska.DeleteAttachments(filePath, plan.FontsToRemove); err != nil {
			return fmt.Errorf("deleting attachments: %w", err)
		}
	}

	return nil
}

func executeChapters(filePath string, plan *FixPlan) error {
	if plan.ChapterKeyframeSnaps.Changed > 0 && len(plan.ChapterKeyframeSnaps.Times) > 0 {
		if err := matroska.RewriteChapterTimestamps(filePath, plan.ChapterKeyframeSnaps.Times); err != nil {
			return fmt.Errorf("rewriting chapter timestamps: %w", err)
		}
	}

	return nil
}

func executeTracksAndTags(filePath string, plan *FixPlan) error {
	if plan.WriteStatistics {
		if err := matroska.AddTrackStatisticsTags(filePath); err != nil {
			return fmt.Errorf("adding track statistics tags: %w", err)
		}
	}

	if plan.ClearCreationTime {
		tagsXML, err := matroska.ExtractTagsXML(filePath)
		if err != nil {
			return fmt.Errorf("extracting tags xml: %w", err)
		}

		stripped, removed := checks.StripCreationTimeTags(tagsXML)
		if len(removed) > 0 {
			if err := matroska.SetTagsXML(filePath, stripped); err != nil {
				return fmt.Errorf("setting tags xml: %w", err)
			}
		}
	}

	if len(plan.TrackEdits) > 0 {
		if err := matroska.SetTrackProperties(filePath, plan.TrackEdits); err != nil {
			return fmt.Errorf("setting track properties: %w", err)
		}
	}

	return nil
}

func executeRemux(filePath string, plan *FixPlan) error {
	if !plan.RemuxRequired {
		return nil
	}

	remuxOpts := matroska.RemuxOptions{
		TrackOrder:          plan.RemuxTrackOrder,
		StripCompressionIDs: plan.RemuxStripCompression,
		RemoveTrackIDs:      plan.RemuxRemoveTracks,
	}

	ui.Println(ui.Muted.Render("Remuxing... this may take a while for large files."))

	if err := matroska.RemuxTracks(filePath, remuxOpts); err != nil {
		return fmt.Errorf("remuxing tracks: %w", err)
	}

	return nil
}

package correct

import (
	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/metadata/mediainfo"
)

// ComputeContainerFixes returns the segment-level ("info") property edits
// needed to satisfy the title, writing-application, and creation-time checks.
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

package correct

import (
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

// FixPlan represents a complete set of proposed modifications to a Matroska file.
type FixPlan struct {
	// 1. Container-level Properties (mkvpropedit)
	ContainerProperties map[string]string // e.g. "title": "", "writing-application": ""
	ClearCreationTime   bool
	WriteStatistics     bool

	// 2. Attachment Edits (mkvpropedit)
	AttachmentRenames map[int]string           // Attachment ID -> New compliant filename
	FontsToAdd        []matroska.AttachmentAdd // Missing font files to attach
	FontsToRemove     []int                    // IDs of unused font attachments to strip

	// 3. Chapter Edits (mkvpropedit)
	ChapterKeyframeSnaps ChapterAlignmentFix // Contains full list of new times

	// 4. Track Metadata Edits (mkvpropedit)
	FlagEdits     []matroska.TrackEdit // Flag changes
	NameEdits     []matroska.TrackEdit // Name changes
	LanguageEdits []matroska.TrackEdit // Language changes

	// 5. Destructive Remux Operations (mkvmerge)
	RemuxRequired         bool
	RemuxTrackOrder       []int // Track IDs in their new desired order
	RemuxRemoveTracks     []int // Track IDs to delete (e.g., empty tracks, unwanted audio)
	RemuxStripCompression []int // Track IDs that need zlib compression stripped
}

// NewFixPlan creates an empty FixPlan.
func NewFixPlan() *FixPlan {
	return &FixPlan{
		ContainerProperties:   make(map[string]string),
		AttachmentRenames:     make(map[int]string),
		FontsToAdd:            make([]matroska.AttachmentAdd, 0),
		FontsToRemove:         make([]int, 0),
		ChapterKeyframeSnaps:  ChapterAlignmentFix{},
		FlagEdits:             make([]matroska.TrackEdit, 0),
		NameEdits:             make([]matroska.TrackEdit, 0),
		LanguageEdits:         make([]matroska.TrackEdit, 0),
		RemuxTrackOrder:       make([]int, 0),
		RemuxRemoveTracks:     make([]int, 0),
		RemuxStripCompression: make([]int, 0),
	}
}

// IsEmpty returns true if the plan contains no modifications.
func (p *FixPlan) IsEmpty() bool {
	return !p.hasContainerEdits() && !p.hasTrackEdits() && !p.RemuxRequired
}

func (p *FixPlan) hasContainerEdits() bool {
	return len(p.ContainerProperties) > 0 ||
		p.ClearCreationTime ||
		p.WriteStatistics ||
		len(p.AttachmentRenames) > 0 ||
		len(p.FontsToAdd) > 0 ||
		len(p.FontsToRemove) > 0 ||
		p.ChapterKeyframeSnaps.Changed > 0
}

func (p *FixPlan) hasTrackEdits() bool {
	return len(p.FlagEdits) > 0 || len(p.NameEdits) > 0 || len(p.LanguageEdits) > 0
}

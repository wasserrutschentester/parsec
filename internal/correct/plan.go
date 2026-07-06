package correct

import (
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

// Options controls which fixes are applied and how interactive decisions are
// handled.
type Options struct {
	DryRun     bool
	Remux      bool
	Unattended bool
	ImdbID     string
	TmdbID     int
	TvdbID     int
}

// FixPlan represents a complete set of proposed modifications to a Matroska file.
type FixPlan struct {
	// 1. Container Properties (mkvpropedit)
	ContainerProperties []ContainerPropertyEdit // e.g. Title
	ClearCreationTime   bool
	WriteStatistics     bool

	// 2. Font Attachments (mkvpropedit)
	AttachmentRenames []AttachmentRename      // Rename non-compliant fonts
	FontsToAdd        []MissingFontAttachment // Missing fonts to attach
	FontsToRemove     []AttachmentRemove      // Unused fonts to removents to strip

	// 3. Chapter Edits (mkvpropedit)
	ChapterKeyframeSnaps ChapterAlignmentFix // Contains full list of new times

	// 4. Track Metadata Edits (mkvpropedit)
	FlagEdits     []matroska.TrackEdit // Flag changes
	NameEdits     []matroska.TrackEdit // Name changes
	LanguageEdits []matroska.TrackEdit // Language changes

	// 5. Destructive Remux Operations (mkvmerge)
	RemuxRequired         bool
	RemuxTrackOrder       []int              // Track IDs in their new desired order
	RemuxRemoveTracks     []RemovalCandidate // Track IDs to delete (e.g., empty tracks, unwanted audio)
	RemuxStripCompression []int              // Track IDs that need zlib compression stripped
}

// NewFixPlan creates an empty FixPlan.
func NewFixPlan() *FixPlan {
	return &FixPlan{
		ContainerProperties:   make([]ContainerPropertyEdit, 0),
		AttachmentRenames:     make([]AttachmentRename, 0),
		FontsToAdd:            make([]MissingFontAttachment, 0),
		FontsToRemove:         make([]AttachmentRemove, 0),
		ChapterKeyframeSnaps:  ChapterAlignmentFix{},
		FlagEdits:             make([]matroska.TrackEdit, 0),
		NameEdits:             make([]matroska.TrackEdit, 0),
		LanguageEdits:         make([]matroska.TrackEdit, 0),
		RemuxTrackOrder:       make([]int, 0),
		RemuxRemoveTracks:     make([]RemovalCandidate, 0),
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

// ContainerPropertyEdit represents a change to a single container-level property.
type ContainerPropertyEdit struct {
	Key      string
	OldValue string
	NewValue string
}

// AttachmentRename represents a rename of a font attachment.
type AttachmentRename struct {
	ID      int
	OldName string
	NewName string
}

// AttachmentRemove represents the removal of an unused font attachment.
type AttachmentRemove struct {
	ID   int
	Name string
}

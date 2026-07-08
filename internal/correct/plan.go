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
	// Safe, in-place edits (mkvpropedit)
	Metadata MetadataPlan `json:"metadata"`

	// Destructive file-rewriting operations (mkvmerge)
	Remux RemuxPlan `json:"remux"`

	// OriginalLanguage holds the resolved original language of the media file,
	// used for display logic and context during review.
	OriginalLanguage string `json:"original_language"`
}

// MetadataPlan defines safe, in-place edits executed via mkvpropedit.
type MetadataPlan struct {
	Container   ContainerPlan        `json:"container"`
	Attachments AttachmentPlan       `json:"attachments"`
	Chapters    ChapterPlan          `json:"chapters"`
	Tracks      []matroska.TrackEdit `json:"tracks"`
}

// ContainerPlan describes updates to container-level metadata properties.
type ContainerPlan struct {
	Properties        []ContainerPropertyEdit `json:"properties"`
	ClearCreationTime bool                    `json:"clear_creation_time"`
	WriteStatistics   bool                    `json:"write_statistics"`
}

// AttachmentPlan describes additions, removals, and renames of font attachments.
type AttachmentPlan struct {
	Renames  []AttachmentRename      `json:"renames"`
	ToAdd    []MissingFontAttachment `json:"to_add"`
	ToRemove []AttachmentRemove      `json:"to_remove"`
}

// ChapterPlan describes timeline alignment fixes for chapters.
type ChapterPlan struct {
	KeyframeSnaps ChapterAlignmentFix `json:"keyframe_snaps"`
}

// ChapterSnapEvent describes an alignment fix for a specific chapter.
type ChapterSnapEvent struct {
	ChapterNum       int    `json:"chapter_num"`
	Name             string `json:"name"`
	OriginalTime     int64  `json:"original_time"`
	PreviousKeyframe int64  `json:"previous_keyframe"`
	NextKeyframe     int64  `json:"next_keyframe"`
	DefaultDuration  int64  `json:"default_duration"`
}

// RemuxPlan describes destructive file operations executed via mkvmerge.
type RemuxPlan struct {
	Required         bool               `json:"required"`
	TrackOrder       []int              `json:"track_order"`
	RemoveTracks     []RemovalCandidate `json:"remove_tracks"`
	StripCompression []int              `json:"strip_compression"`
}

// NewFixPlan creates an empty FixPlan.
func NewFixPlan() *FixPlan {
	return &FixPlan{
		Metadata: MetadataPlan{
			Container: ContainerPlan{
				Properties: make([]ContainerPropertyEdit, 0),
			},
			Attachments: AttachmentPlan{
				Renames:  make([]AttachmentRename, 0),
				ToAdd:    make([]MissingFontAttachment, 0),
				ToRemove: make([]AttachmentRemove, 0),
			},
			Chapters: ChapterPlan{
				KeyframeSnaps: ChapterAlignmentFix{},
			},
			Tracks: make([]matroska.TrackEdit, 0),
		},
		Remux: RemuxPlan{
			TrackOrder:       make([]int, 0),
			RemoveTracks:     make([]RemovalCandidate, 0),
			StripCompression: make([]int, 0),
		},
	}
}

// IsEmpty returns true if the plan contains no modifications.
func (p *FixPlan) IsEmpty() bool {
	return !p.hasContainerEdits() && !p.hasTrackEdits() && !p.Remux.Required
}

func (p *FixPlan) hasContainerEdits() bool {
	return len(p.Metadata.Container.Properties) > 0 ||
		p.Metadata.Container.ClearCreationTime ||
		p.Metadata.Container.WriteStatistics ||
		len(p.Metadata.Attachments.Renames) > 0 ||
		len(p.Metadata.Attachments.ToAdd) > 0 ||
		len(p.Metadata.Attachments.ToRemove) > 0 ||
		p.Metadata.Chapters.KeyframeSnaps.Changed > 0
}

func (p *FixPlan) hasTrackEdits() bool {
	return len(p.Metadata.Tracks) > 0
}

// ContainerPropertyEdit represents a change to a single container-level property.
type ContainerPropertyEdit struct {
	Key      string `json:"key"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
	Reason   string `json:"reason"`
}

// AttachmentRename represents a rename of a font attachment.
type AttachmentRename struct {
	ID             int    `json:"id"`
	OldName        string `json:"old_name"`
	NewName        string `json:"new_name"`
	FullName       string `json:"full_name"`
	PostScriptName string `json:"postscript_name"`
}

// AttachmentRemove represents the removal of an unused font attachment.
type AttachmentRemove struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	FullName  string `json:"full_name"`
	SizeBytes int64  `json:"size_bytes"`
	Reason    string `json:"reason"`
}

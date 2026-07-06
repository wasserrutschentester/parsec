// Package resolve provides a unified pipeline for bootstrapping MediaInfo, EBML, and Metadata.
package resolve

import (
	"fmt"

	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/filename"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/metadata/mediainfo"
	"codeberg.org/upPollo/parsec/internal/ui"
)

// Options configuration for resolving the initial metadata.
type Options struct {
	FilePath string

	// Explicit MDB ID overrides (from CLI flags or config)
	ImdbID string
	TmdbID int
	TvdbID int

	// Explicit Type override (from CLI flags)
	IsTVSet    bool
	IsMovieSet bool

	// Whether to parse EBML. This is usually true for check and rename but
	// can be false for pure metadata resolution if EBML parsing isn't needed.
	ParseEBML bool
}

// Result contains the fully resolved metadata along with the raw parsed structures.
type Result struct {
	Meta      *metadata.Metadata
	MediaInfo *mediainfo.MediaInfo
	Ebml      *matroska.EbmlMetadata // nil if ParseEBML is false or parsing failed
	EbmlErr   error                  // preserves the EBML error if ParseEBML is true
}

// Metadata orchestrates the pipeline to resolve the final metadata state from multiple sources:
// 1. Filename parsing
// 2. MediaInfo parsing & merging
// 3. MDB IDs extraction from tags
// 4. EBML parsing (for visual impaired tracks)
// 5. CLI overrides
func Metadata(opts Options) (*Result, error) {
	filenameNoExt := filename.GetBaseName(opts.FilePath)

	// 1. Parse filename for initial metadata
	meta := filename.Parse(filenameNoExt)

	// 2. Get MediaInfo and merge
	mi, err := mediainfo.Get(opts.FilePath)
	if err != nil {
		ui.PrintDebug(fmt.Sprintf("Could not read MediaInfo for %s: %v", ui.AnonymizePath(opts.FilePath), err))
		// Continue even if mediainfo fails, we still want to apply overrides
	} else {
		meta.Override(mi.GetMetadata())
		applyMediaInfoTags(meta, mi, opts)
	}

	// 4. Optionally parse EBML
	var ebml *matroska.EbmlMetadata

	var ebmlErr error

	if opts.ParseEBML {
		ebml, ebmlErr = matroska.GetEbmlMetadata(opts.FilePath)
		if ebmlErr == nil && ebml.HasVisualImpairedAudio() && !meta.HasAudioDesc {
			meta.HasAudioDesc = true
		}
	}

	// 5. Apply explicit overrides (CLI flags)
	if opts.ImdbID != "" {
		meta.ImdbID = opts.ImdbID
	}

	if opts.TmdbID != 0 {
		meta.TmdbID = opts.TmdbID
	}

	if opts.TvdbID != 0 {
		meta.TvdbID = opts.TvdbID
	}

	return &Result{
		Meta:      meta,
		MediaInfo: mi,
		Ebml:      ebml,
		EbmlErr:   ebmlErr,
	}, err // Return MediaInfo error as the overall error, callers can choose to ignore
}

func applyMediaInfoTags(meta *metadata.Metadata, mi *mediainfo.MediaInfo, opts Options) {
	tagImdb, tagTmdb, tagTvdb, tagIsTV := mi.GetMdbIDs()

	if meta.ImdbID == "" {
		meta.ImdbID = tagImdb
	}

	if meta.TmdbID == 0 {
		meta.TmdbID = tagTmdb
	}

	if meta.TvdbID == 0 {
		meta.TvdbID = tagTvdb
	}

	if !opts.IsTVSet && !opts.IsMovieSet && tagIsTV {
		meta.IsTV = true
	}
}

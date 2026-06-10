package cmd

import (
	"fmt"

	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/ui"
	"github.com/spf13/cobra"
)

var (
	// Metadata
	titleFlag        string
	yearFlag         int
	seasonFlag       int
	episodeFlag      int
	dateFlag         string
	episodeTitleFlag string
	cutEditionFlag   string
	hdrFlag          string
	// P2P
	serviceFlag     string
	sourceFlag      string
	isRepackFlag    bool
	isSubbedFlag    bool
	isAudioDescFlag bool
	groupFlag       string
	// MDB IDs
	isTVFlag    bool
	isMovieFlag bool
	imdbIDFlag  string
	tmdbIDFlag  int
	tvdbIDFlag  int
	// other
	dryRunFlag     bool
	unattendedFlag bool
	verboseFlag    bool
	debugFlag      bool
	presetFlag     string
	noCacheFlag    bool
	releasesFlag   bool
	bestFlag       bool
)

// nolint:funlen
func applyMetadataFlags(cmd *cobra.Command, meta *metadata.Metadata) {
	if titleFlag != "" {
		meta.Title = titleFlag
	}

	if yearFlag != 0 {
		meta.Year = yearFlag
	}

	if seasonFlag != 0 {
		meta.Season = seasonFlag
	}

	if episodeFlag != 0 {
		meta.Episode = episodeFlag
	}

	if dateFlag != "" {
		meta.Date = dateFlag
	}

	if episodeTitleFlag != "" {
		meta.EpisodeTitle = episodeTitleFlag
	}

	if cutEditionFlag != "" {
		meta.CutEdition = cutEditionFlag
	}

	if hdrFlag != "" {
		meta.HDR = hdrFlag
	}

	if serviceFlag != "" {
		meta.Service = serviceFlag
	}

	if sourceFlag != "" {
		meta.Source = sourceFlag
	}

	if isRepackFlag {
		meta.Repack = isRepackFlag
	}

	if isSubbedFlag {
		meta.Subbed = isSubbedFlag
	}

	if isAudioDescFlag {
		meta.HasAudioDesc = isAudioDescFlag
	}

	if groupFlag != "" {
		meta.Group = groupFlag
	}

	if cmd.Flags().Changed("tv") {
		meta.IsTV = isTVFlag
	} else if cmd.Flags().Changed("movie") {
		meta.IsTV = !isMovieFlag
	}

	if imdbIDFlag != "" {
		meta.ImdbID = imdbIDFlag
	}

	if tmdbIDFlag != 0 {
		meta.TmdbID = tmdbIDFlag
	}

	if tvdbIDFlag != 0 {
		meta.TvdbID = tvdbIDFlag
	}

	ui.PrintDebug(fmt.Sprintf("Set Flag overrides: %+v", meta))
}

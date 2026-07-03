package correct

import (
	"fmt"

	"golang.org/x/text/language"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/mdb"
	mdbSearch "codeberg.org/upPollo/parsec/internal/mdb/search"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/filename"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/metadata/mediainfo"
	"codeberg.org/upPollo/parsec/internal/ui"
)

func lookupOriginalLanguage(filePath string, tracks []matroska.EbmlTrack, opts Options) string {
	if !needsOriginalLanguageForUnwantedAudio(tracks) {
		return ""
	}

	if override := config.GetOriginalLanguage(); override != "" {
		ui.Println(ui.Info.Render("Using original_language override: " + override))

		return override
	}

	if opts.Unattended || !ui.IsTerminal() {
		return ""
	}

	meta := buildFixMetadata(filePath, opts)

	result, err := mdbSearch.InteractiveSearch(meta, opts.Unattended)
	if err != nil {
		ui.PrintDebug(fmt.Sprintf("skipping unwanted-language audio removal for %s: %v", ui.AnonymizePath(filePath), err))
		ui.PrintWarning("Skipping unwanted-language audio removal: original language unavailable from MDB.")

		return ""
	}

	if result == nil || language.Make(result.OriginalLanguage) == language.Und {
		ui.PrintWarning("Skipping unwanted-language audio removal: original language unavailable from MDB.")

		return ""
	}

	mdb.PrintCompactResult(*result)

	return result.OriginalLanguage
}

func needsOriginalLanguageForUnwantedAudio(tracks []matroska.EbmlTrack) bool {
	if !config.IsCheckEnabled(config.CheckMdbUnwantedAudioLang) {
		return false
	}

	preferred := language.Make(config.GetPreferredLanguage())
	allowedWithoutMDB := map[language.Tag]bool{
		preferred:            true,
		language.Und:         true,
		language.Make("mul"): true,
		language.Make("zxx"): true,
		language.Make(""):    true,
	}

	for _, track := range tracks {
		if track.Type != "audio" {
			continue
		}

		if !allowedWithoutMDB[language.Make(track.Properties.Language)] {
			return true
		}
	}

	return false
}

func buildFixMetadata(filePath string, opts Options) *metadata.Metadata {
	meta := filename.Parse(filename.GetBaseName(filePath))

	mi, err := mediainfo.Get(filePath)
	if err != nil {
		ui.PrintDebug(fmt.Sprintf("could not read MediaInfo for MDB lookup on %s: %v", ui.AnonymizePath(filePath), err))
		applyMdbIDOverrides(meta, opts)

		return meta
	}

	meta.Override(mi.GetMetadata())
	applyMdbIDsFromMediaInfo(meta, mi)
	applyMdbIDOverrides(meta, opts)

	return meta
}

func applyMdbIDsFromMediaInfo(meta *metadata.Metadata, mi *mediainfo.MediaInfo) {
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

	if tagIsTV {
		meta.IsTV = true
	}
}

func applyMdbIDOverrides(meta *metadata.Metadata, opts Options) {
	if opts.ImdbID != "" {
		meta.ImdbID = opts.ImdbID
	}

	if opts.TmdbID != 0 {
		meta.TmdbID = opts.TmdbID
	}

	if opts.TvdbID != 0 {
		meta.TvdbID = opts.TvdbID
		meta.IsTV = true
	}
}

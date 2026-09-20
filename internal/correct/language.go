package correct

import (
	"fmt"
	"slices"

	"golang.org/x/text/language"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/mdb"
	mdbSearch "codeberg.org/upPollo/parsec/internal/mdb/search"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/metadata/resolve"
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

	res, _ := resolve.Metadata(resolve.Options{
		FilePath: filePath,
		ImdbID:   opts.ImdbID,
		TmdbID:   opts.TmdbID,
		TvdbID:   opts.TvdbID,
	})
	meta := res.Meta

	result, err := mdbSearch.InteractiveSearch(meta, opts.Unattended, false)
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

	return slices.ContainsFunc(tracks, func(track matroska.EbmlTrack) bool {
		return track.Type == "audio" && !allowedWithoutMDB[language.Make(track.Properties.Language)]
	})
}

package fix

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

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

const originalLanguageMaxLength = 3

var errInvalidOriginalLanguage = errors.New("invalid OV override")

func lookupOriginalLanguage(filePath string, tracks []matroska.EbmlTrack, opts Options) string {
	if !needsOriginalLanguageForUnwantedAudio(tracks) {
		return ""
	}

	if opts.OriginalLanguage != "" {
		ui.Println(ui.Info.Render("Using OV override: " + opts.OriginalLanguage))

		return opts.OriginalLanguage
	}

	if opts.Unattended || !ui.IsTerminal() {
		return ""
	}

	meta := buildMdbLookupMetadata(filePath, opts)

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

func normalizeOriginalLanguageCode(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 2 || len(trimmed) > originalLanguageMaxLength {
		return "", invalidOriginalLanguageError(value)
	}

	for _, r := range trimmed {
		if !unicode.IsLetter(r) {
			return "", invalidOriginalLanguageError(value)
		}
	}

	tag, err := language.Parse(trimmed)
	if err != nil || tag == language.Und {
		return "", invalidOriginalLanguageError(value)
	}

	base, confidence := tag.Base()
	if confidence == language.No || len(base.String()) != 2 {
		return "", invalidOriginalLanguageError(value)
	}

	return base.String(), nil
}

func invalidOriginalLanguageError(value string) error {
	return fmt.Errorf("%w %q: expected a 2- or 3-letter language code", errInvalidOriginalLanguage, value)
}

func needsOriginalLanguageForUnwantedAudio(tracks []matroska.EbmlTrack) bool {
	if !config.IsCheckEnabled("mdb_unwanted_audio_lang") {
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

func buildMdbLookupMetadata(filePath string, opts Options) *metadata.Metadata {
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

package mdb

// TitleType represents the standardized classification for media titles.
type TitleType string

const (
	// TitleTypeUnknown represents an unspecified or unknown title type.
	TitleTypeUnknown TitleType = ""
	// TitleTypeMovie represents feature-length theatrical or streaming films.
	TitleTypeMovie TitleType = "movie"
	// TitleTypeTVSeries represents multi-episode television or web series.
	TitleTypeTVSeries TitleType = "tvSeries"
	// TitleTypeTVMiniSeries represents limited standalone mini-series.
	TitleTypeTVMiniSeries TitleType = "tvMiniSeries"
	// TitleTypeTVSpecial represents standalone television specials.
	TitleTypeTVSpecial TitleType = "tvSpecial"
	// TitleTypeTVMovie represents made-for-television films.
	TitleTypeTVMovie TitleType = "tvMovie"
	// TitleTypeTVShort represents short television productions.
	TitleTypeTVShort TitleType = "tvShort"
	// TitleTypeShort represents theatrical short films.
	TitleTypeShort TitleType = "short"
	// TitleTypeVideo represents direct-to-video / home video releases.
	TitleTypeVideo TitleType = "video"
	// TitleTypeVideoGame represents interactive video games.
	TitleTypeVideoGame TitleType = "videoGame"
	// TitleTypePodcastSeries represents episodic audio or video podcasts.
	TitleTypePodcastSeries TitleType = "podcastSeries"
	// TitleTypePodcastEpisode represents individual podcast episodes.
	TitleTypePodcastEpisode TitleType = "podcastEpisode"
	// TitleTypeMusicVideo represents official music videos.
	TitleTypeMusicVideo TitleType = "musicVideo"
	// TitleTypeTVEpisode represents individual television episodes.
	TitleTypeTVEpisode TitleType = "tvEpisode"
	// TitleTypeTVPilot represents standalone television pilot episodes.
	TitleTypeTVPilot TitleType = "tvPilot"
	// TitleTypeAudiobook represents audiobooks.
	TitleTypeAudiobook TitleType = "audiobook"
)

// IsExcludedFromSearch returns true for non-broadcast/cinematic formats (e.g. music videos, podcasts, video games, audiobooks)
// that should not be returned in general media searches, or for TV episodes when searching for a TV series.
func (t TitleType) IsExcludedFromSearch(isTV ...bool) bool {
	if t == TitleTypePodcastSeries || t == TitleTypePodcastEpisode || t == TitleTypeMusicVideo || t == TitleTypeVideoGame || t == TitleTypeAudiobook {
		return true
	}

	if len(isTV) > 0 && isTV[0] && t == TitleTypeTVEpisode {
		return true
	}

	return false
}

// IsSpecific returns true if the title type is a specific subtype rather than generic movie or tvSeries.
func (t TitleType) IsSpecific() bool {
	switch t {
	case TitleTypeTVMiniSeries, TitleTypeTVSpecial, TitleTypeTVMovie, TitleTypeTVShort, TitleTypeTVEpisode, TitleTypeTVPilot, TitleTypeShort, TitleTypeVideo:
		return true
	default:
		return false
	}
}

var titleTypeDisplayNames = map[TitleType]string{
	TitleTypeMovie:          "Movie",
	TitleTypeTVSeries:       "TV Series",
	TitleTypeTVMiniSeries:   "Mini-Series",
	TitleTypeTVSpecial:      "TV Special",
	TitleTypeTVMovie:        "TV Movie",
	TitleTypeTVShort:        "TV Short",
	TitleTypeShort:          "Short Film",
	TitleTypeVideo:          "Direct-to-Video",
	TitleTypeVideoGame:      "Video Game",
	TitleTypePodcastSeries:  "Podcast Series",
	TitleTypePodcastEpisode: "Podcast Episode",
	TitleTypeMusicVideo:     "Music Video",
	TitleTypeTVEpisode:      "TV Episode",
	TitleTypeTVPilot:        "TV Pilot",
	TitleTypeAudiobook:      "Audiobook",
}

// FormatTitleType returns a human-readable display string for a TitleType.
func FormatTitleType(t TitleType) string {
	if name, ok := titleTypeDisplayNames[t]; ok {
		return name
	}

	return string(t)
}

// ShouldDisplayTitleType returns true if the title type should be shown in the identify card.
// It is displayed if it is not one of the standard types (movie for movies, tvSeries for TV shows)
// or contradicts the isTV classification (e.g. a TV show/episode classified as movie on TMDB).
func ShouldDisplayTitleType(titleType TitleType, isTV bool) bool {
	return titleType != "" && ((isTV && titleType != TitleTypeTVSeries) || (!isTV && titleType != TitleTypeMovie))
}

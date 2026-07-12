package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

// expandArgs takes a list of paths and expands any directories into a list of Matroska files recursively.
func expandArgs(args []string) []string {
	var expanded []string

	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			expanded = append(expanded, arg)

			continue
		}

		if info.IsDir() {
			_ = filepath.WalkDir(arg, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}

				if filepath.Ext(path) == ".mkv" && matroska.CheckForMatroska(path) == nil {
					expanded = append(expanded, path)
				}

				return nil
			})
		} else {
			expanded = append(expanded, arg)
		}
	}

	return expanded
}

// isCompletionCommand reports whether cmd is one of Cobra's internal shell
// completion commands. Used to suppress stdout output that would otherwise
// corrupt the completion stream.
func isCompletionCommand(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "__complete", "__completeNoDesc":
		return true
	default:
		return false
	}
}

// completeFiles returns shell completion candidates for file arguments.
// It lists entries inside the directory implied by toComplete, keeping
// subdirectories (so the user can navigate into them) and files whose
// extension matches one of the provided exts (without the leading dot).
//
// ShellCompDirectiveNoFileComp is returned so the shell does not add its
// own unfiltered file suggestions on top — this gives consistent behaviour
// across bash, zsh, and fish (fish ignores ShellCompDirectiveFilterFileExt).
func completeFiles(toComplete string, exts ...string) ([]string, cobra.ShellCompDirective) {
	dir := filepath.Dir(toComplete)
	if toComplete == "" {
		dir = "."
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Determine the basename the user has typed so far in the current directory.
	// filepath.Base("") returns "." which would incorrectly enable hidden-file
	// matching, so we handle the empty case explicitly.
	typedPrefix := ""
	if toComplete != "" {
		typedPrefix = filepath.Base(toComplete)
	}

	var completions []string

	for _, e := range entries {
		// Skip hidden entries unless the user has already typed a dot prefix,
		// matching the default shell behaviour.
		if strings.HasPrefix(e.Name(), ".") && !strings.HasPrefix(typedPrefix, ".") {
			continue
		}

		var candidate string
		if dir == "." {
			candidate = e.Name()
		} else {
			candidate = filepath.Join(dir, e.Name())
		}

		if e.IsDir() {
			completions = append(completions, candidate+string(filepath.Separator))

			continue
		}

		if hasAllowedExt(e.Name(), exts) {
			completions = append(completions, candidate)
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

// hasAllowedExt reports whether name's extension (case-insensitive, without
// the leading dot) matches any entry in exts.
func hasAllowedExt(name string, exts []string) bool {
	ext := strings.TrimPrefix(filepath.Ext(name), ".")

	for _, allowed := range exts {
		if strings.EqualFold(ext, allowed) {
			return true
		}
	}

	return false
}

// completeSources returns common source values with description hints.
func completeSources(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return formatMapCompletions(sourceOptions), cobra.ShellCompDirectiveNoFileComp
}

// completeHdrs returns standard HDR formats with description hints.
func completeHdrs(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return formatMapCompletions(hdrOptions), cobra.ShellCompDirectiveNoFileComp
}

// completeLanguages returns common ISO 639-1 language codes with description hints.
func completeLanguages(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return formatMapCompletions(languageOptions), cobra.ShellCompDirectiveNoFileComp
}

// completeServices returns common streaming service tags with description hints.
func completeServices(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return formatMapCompletions(serviceOptions), cobra.ShellCompDirectiveNoFileComp
}

// completeCutEditions returns common release editions/cuts with description hints,
// formatted using the user's configured word separator.
func completeCutEditions(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	sep := config.GetWordSeparator()

	formattedOptions := make(map[string]string, len(cutEditionOptions))
	for k, v := range cutEditionOptions {
		formattedKey := strings.ReplaceAll(k, " ", sep)
		formattedOptions[formattedKey] = v
	}

	return formatMapCompletions(formattedOptions), cobra.ShellCompDirectiveNoFileComp
}

// completeGroups returns the user's default configured group with a description hint.
func completeGroups(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	initConfig()

	g := config.GetGroup()
	if g == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return []string{g + "\tConfigured default group"}, cobra.ShellCompDirectiveNoFileComp
}

// formatMapCompletions converts a map of completions to a sorted slice of "key\tdescription" strings.
func formatMapCompletions(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	completions := make([]string, len(keys))
	for i, k := range keys {
		completions[i] = k + "\t" + m[k]
	}

	return completions
}

var sourceOptions = map[string]string{
	"BluRay": "Blu-ray disc source",
	"DVD5":   "Single layer DVD fitting around 4.7 GB",
	"DVD9":   "Double layer DVD fitting around 8.5 GB",
	"DVDRip": "DVD encode",
	"HDDVD":  "High Definition DVD",
	"HDTV":   "High Definition Television broadcast",
	"dTV":    "Standard Definition Digital TV broadcast",
	"WEB-DL": "Web Download (untouched stream from streaming services)",
	"WEBRip": "Web Rip (transcoded/re-encoded stream from streaming services)",
}

var hdrOptions = map[string]string{
	"DV":        "Dolby Vision",
	"HDR":       "HDR10 (static metadata)",
	"HDR10Plus": "HDR10+ (dynamic metadata)",
	"HLG":       "Hybrid Log-Gamma",
	"PQ10":      "PQ10 format",
}

var languageOptions = map[string]string{
	"ar": "Arabic",
	"cs": "Czech",
	"da": "Danish",
	"de": "German",
	"en": "English",
	"es": "Spanish",
	"fi": "Finnish",
	"fr": "French",
	"hi": "Hindi",
	"hu": "Hungarian",
	"it": "Italian",
	"ja": "Japanese",
	"ko": "Korean",
	"nl": "Dutch",
	"no": "Norwegian",
	"pl": "Polish",
	"pt": "Portuguese",
	"ro": "Romanian",
	"ru": "Russian",
	"sv": "Swedish",
	"tr": "Turkish",
	"uk": "Ukrainian",
	"zh": "Chinese",
}

var serviceOptions = map[string]string{
	"AMZN": "Amazon Prime Video",
	"ARD":  "ARD Mediathek",
	"ATVP": "Apple TV+",
	"CR":   "Crunchyroll",
	"DSNP": "Disney+",
	"HMAX": "HBO Max",
	"KiKA": "KiKA",
	"NF":   "Netflix",
	"PCOK": "Peacock",
	"PMTP": "Paramount+",
	"RTLP": "RTL+",
	"SHO":  "Showtime",
	"ZDF":  "ZDF Mediathek",
	"iT":   "iTunes",
}

var cutEditionOptions = map[string]string{
	"3D":              "3D Version",
	"3D HOU":          "3D Half Over-Under",
	"3D HSBS":         "3D Half Side-by-Side",
	"3D SBS":          "3D Side-by-Side",
	"4K REMASTERED":   "Remastered in 4K",
	"CRITERION":       "Criterion Collection",
	"DIRECTOR'S CUT":  "Director's Cut",
	"EXTENDED":        "Extended Edition",
	"IMAX":            "IMAX Version",
	"IMAX Enhanced":   "IMAX Enhanced Version",
	"Open Matte":      "Open Matte aspect ratio",
	"REMASTERED":      "Remastered Version",
	"SPECIAL EDITION": "Special Edition",
	"THEATRICAL":      "Theatrical Version",
	"UNCENSORED":      "Uncensored Version",
	"UNRATED":         "Unrated Version",
}

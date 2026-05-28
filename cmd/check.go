package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"codeberg.org/n0ne/parsec/internal/checks"
	"codeberg.org/n0ne/parsec/internal/config"
	"codeberg.org/n0ne/parsec/internal/metadata/filename"
	"codeberg.org/n0ne/parsec/internal/metadata/matroska"
	"codeberg.org/n0ne/parsec/internal/metadata/mediainfo"
	"codeberg.org/n0ne/parsec/internal/ui"

	"github.com/spf13/cobra"
)

type issueGroup struct {
	Category string               `json:"category"`
	Results  []checks.CheckResult `json:"results"`
}

type checkReport struct {
	File          string       `json:"file"`
	Passed        bool         `json:"passed"`
	ReleaseName   string       `json:"filename"`
	GeneratedName string       `json:"generated_name"`
	Issues        []issueGroup `json:"issues"`
}

var jsonOutputFlag bool

var checkCmd = &cobra.Command{
	Use:   "check [file]",
	Short: "Check if the file fits the specification",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ui.IsSilent = jsonOutputFlag
		filePath := args[0]
		filenameNoExt := filename.GetBaseName(filePath)

		ui.Println(ui.Header.Render("Parsec File Check"))
		ui.Println(ui.LabelValue("Current Name:", filenameNoExt))

		var allIssues []issueGroup

		// 1. Filename Basic Checks
		var fnIssues []checks.CheckResult
		if config.IsCheckEnabled("filename_characters") {
			if msg := filename.CheckAllowedCharacters(filenameNoExt); msg != "" {
				fnIssues = append(fnIssues, checks.CheckResult{Warning: msg, Passed: false})
			}
		}
		if config.IsCheckEnabled("filename_sequences") {
			if msg := filename.CheckCharacterSequences(filenameNoExt); msg != "" {
				fnIssues = append(fnIssues, checks.CheckResult{Warning: msg, Passed: false})
			}
		}

		match := filename.Parse(filenameNoExt)
		if match.String() != filenameNoExt {
			fnIssues = append(fnIssues, checks.CheckResult{Warning: ui.Warning.Render("Some tags weren't parsed correctly from the filename"), Passed: false})
			fnIssues = append(fnIssues, checks.CheckResult{Warning: ui.LabelValue("Parsed Name:", match.String()), Passed: false})
		}
		if len(fnIssues) > 0 {
			allIssues = append(allIssues, issueGroup{"FILENAME", fnIssues})
		}

		// 2. MediaInfo & EBML Checks
		mi, err := mediainfo.Get(filePath)
		if err != nil {
			ui.PrintError(fmt.Sprintf("Error getting mediainfo: %v", err))
			return
		}
		mediaMeta := mi.GetMetadata()
		updated := match.Override(mediaMeta, false)

		ebml, ebmlErr := matroska.GetEbmlMetadata(filePath)
		if ebmlErr == nil {
			if ebml.HasVisualImpairedAudio() && !match.HasAudioDesc {
				match.HasAudioDesc = true
				updated = true
			}
		}

		if updated {
			ui.Println(ui.Info.Render("\nUpdates applied from MediaInfo/EBML:"))
			ui.Println(ui.LabelValue("Generated Name:", match.String()))
		}

		// Collect MediaInfo issues
		miResults := checks.RunMediaInfoChecks(mi, match)
		if len(miResults) > 0 {
			var failed []checks.CheckResult
			for _, r := range miResults {
				if !r.Passed {
					failed = append(failed, r)
				}
			}
			if len(failed) > 0 {
				allIssues = append(allIssues, issueGroup{"MEDIAINFO", failed})
			}
		}

		// Collect EBML issues
		var ebmlIssues []checks.CheckResult
		if ebmlErr == nil {
			ebmlIssues = append(ebmlIssues, matroska.VerifyTrackOrder(ebml.Tracks)...)
			if config.IsCheckEnabled("matroska_default_flags") {
				ebmlIssues = append(ebmlIssues, matroska.CheckDefaultFlags(ebml.Tracks)...)
			}
			if config.IsCheckEnabled("matroska_subtitle_format") {
				ebmlIssues = append(ebmlIssues, matroska.CheckSubtitleFormat(ebml.Tracks)...)
			}
		} else {
			ebmlIssues = append(ebmlIssues, checks.CheckResult{Warning: fmt.Sprintf("Error getting EBML metadata: %v", ebmlErr), Passed: false})
		}
		if len(ebmlIssues) > 0 {
			allIssues = append(allIssues, issueGroup{"MATROSKA", ebmlIssues})
		}

		// 3. Generic Checks
		genericResults := checks.RunGenericChecks(match)
		if len(genericResults) > 0 {
			var failed []checks.CheckResult
			for _, r := range genericResults {
				if !r.Passed {
					failed = append(failed, r)
				}
			}
			if len(failed) > 0 {
				allIssues = append(allIssues, issueGroup{"GENERIC", failed})
			}
		}

		// 4. MDB Checks
		tagImdb, tagTmdb, tagTvdb, tagIsTV := mi.GetMdbIDs()
		if match.ImdbID == "" {
			match.ImdbID = tagImdb
		}
		if match.TmdbID == 0 {
			match.TmdbID = tagTmdb
		}
		if match.TvdbID == 0 {
			match.TvdbID = tagTvdb
		}
		if !cmd.Flags().Changed("tv") && !cmd.Flags().Changed("movie") && tagIsTV {
			match.IsTV = true
		}

		if imdbIDFlag != "" {
			match.ImdbID = imdbIDFlag
		}
		if tmdbIDFlag != 0 {
			match.TmdbID = tmdbIDFlag
		}
		if tvdbIDFlag != 0 {
			match.TvdbID = tvdbIDFlag
		}

		mdbResults := checks.RunMdbChecks(mi, match)
		if len(mdbResults) > 0 {
			var failed []checks.CheckResult
			for _, r := range mdbResults {
				if !r.Passed {
					failed = append(failed, r)
				}
			}
			if len(failed) > 0 {
				allIssues = append(allIssues, issueGroup{"MDB", failed})
			}
		}

		// Final Output
		if jsonOutputFlag {
			report := checkReport{
				File:          filePath,
				Passed:        len(allIssues) == 0,
				ReleaseName:   filenameNoExt,
				GeneratedName: match.String(),
				Issues:        allIssues,
			}

			var data []byte
			var err error
			data, err = json.MarshalIndent(report, "", "  ")
			if err != nil {
				ui.PrintError(fmt.Sprintf("Error generating output: %v", err))
				os.Exit(1)
			}
			fmt.Println(string(data))
			return
		}

		if len(allIssues) == 0 {
			ui.Println("\n" + ui.IconCheck + ui.Success.Render(" All checks passed! The file fits the specification."))
		} else {
			totalIssues := 0
			for _, g := range allIssues {
				totalIssues += len(g.Results)
			}
			ui.Println("\n" + ui.IconCross + ui.Error.Render(fmt.Sprintf(" %d issues found:", totalIssues)))
			for _, group := range allIssues {
				ui.Println(ui.ReportSection(group.Category))
				for _, res := range group.Results {
					if len(res.Tracks) > 0 {
						ui.Println("  " + ui.IconWarn + " " + res.Description)
						// Convert checks.TrackCheckResult to the anonymous struct expected by FormatTrackTable
						var uiTracks []struct {
							ID        string
							Type      string
							TypeOrder int
							Codec     string
							Name      string
							Language  string
							Flags     []string
							Warning   string
						}
						for _, t := range res.Tracks {
							uiTracks = append(uiTracks, struct {
								ID        string
								Type      string
								TypeOrder int
								Codec     string
								Name      string
								Language  string
								Flags     []string
								Warning   string
							}{
								ID:        t.ID,
								Type:      t.Type,
								TypeOrder: t.TypeOrder,
								Codec:     t.Codec,
								Name:      t.Name,
								Language:  t.Language,
								Flags:     t.Flags,
								Warning:   t.Warning,
							})
						}
						ui.Println(ui.FormatTrackTable(uiTracks))
					} else {
						ui.Println("  " + ui.IconWarn + " " + res.Warning)
					}
				}
			}
		}
		ui.Println()
	},
}

func countIssues(groups []issueGroup) int {
	count := 0
	for _, group := range groups {
		count += len(group.Results)
	}
	return count
}

func init() {
	rootCmd.AddCommand(checkCmd)
	checkCmd.Flags().IntVar(&tmdbIDFlag, "tmdb", 0, "TMDB ID")
	checkCmd.Flags().IntVar(&tvdbIDFlag, "tvdb", 0, "TVDB ID")
	checkCmd.Flags().StringVar(&imdbIDFlag, "imdb", "", "IMDb ID")
	checkCmd.Flags().BoolVarP(&jsonOutputFlag, "json", "j", false, "Output format (json)")

	idFlags := []string{"imdb", "tmdb", "tvdb"}
	for _, f := range idFlags {
		checkCmd.Flags().SetAnnotation(f, "group", []string{"id"})
	}
}

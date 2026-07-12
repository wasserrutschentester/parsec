package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

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

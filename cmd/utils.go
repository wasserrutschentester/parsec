package cmd

import (
	"os"
	"path/filepath"

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
				if err != nil {
					return nil // ignore errors walking
				}
				if !d.IsDir() && filepath.Ext(path) == ".mkv" {
					if matroska.CheckForMatroska(path) == nil {
						expanded = append(expanded, path)
					}
				}
				return nil
			})
		} else {
			expanded = append(expanded, arg)
		}
	}
	return expanded
}

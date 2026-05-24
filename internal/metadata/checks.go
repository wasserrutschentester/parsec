package metadata

import (
	"fmt"
	"strings"
)

func RunGenericChecks(meta *Metadata) {
	if err := CheckYear(meta); err != nil {
		fmt.Println(err)
	}
	if err := CheckStreaming(meta); err != nil {
		fmt.Println(err)
	}
	if err := CheckTvSpecial(meta); err != nil {
		fmt.Println(err)
	}
}

func CheckYear(meta *Metadata) error {
	if meta.Year == 0 && !meta.IsTV {
		return fmt.Errorf("year is missing for this Movie")
	}

	if meta.Year > 0 && meta.Season > 1900 {
		return fmt.Errorf("The Season already is the year")
	}

	return nil
}

func CheckStreaming(meta *Metadata) error {
	isWeb := strings.Contains(meta.Source, "WEB")
	if isWeb && meta.Service == "" {
		return fmt.Errorf("Streaming Service Tag is missing for WEB source")
	}

	if !isWeb && meta.Service != "" {
		return fmt.Errorf("Streaming Service Tag is not supported for non-WEB source")
	}

	return nil
}

func CheckTvSpecial(meta *Metadata) error {
	if meta.IsTV && meta.Season == 0 {
		if meta.Date == "" {
			return fmt.Errorf("Date is missing for TV Special")
		}
		if meta.EpisodeTitle == "" {
			return fmt.Errorf("Episode Title is missing for TV Special")
		}
	}
	return nil
}

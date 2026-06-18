package filename

import (
	"fmt"
	"reflect"
	"testing"

	"codeberg.org/upPollo/parsec/internal/metadata"
)

func TestReproMultiEpisode(t *testing.T) {
	t.Parallel()

	input := "Kaeptn.Blaubaers.Seemannsgarn.S01E01-E06.Wie.das.Schiff.zur.Klippe.kam.uvm.GERMAN.1080p.ATV.WEB-DL.h264-SLiDE"

	got := Parse(input)

	want := &metadata.Metadata{
		Title:        "Kaeptn.Blaubaers.Seemannsgarn",
		Season:       1,
		Episodes:     []int{1, 2, 3, 4, 5, 6},
		EpisodeTitle: "Wie.das.Schiff.zur.Klippe.kam.uvm",
		Language:     "GERMAN",
		Resolution:   "1080p",
		Service:      "ATV",
		Source:       "WEB-DL",
		VideoCodec:   "h264",
		Group:        "SLiDE",
		IsTV:         true,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse() differences:\n%s", compareMetadataPtr(got, want))
	}
}

func compareMetadataPtr(got, want *metadata.Metadata) string {
	if got == nil || want == nil {
		return fmt.Sprintf("got %v, want %v", got, want)
	}

	var diffs []string

	vGot := reflect.ValueOf(*got)
	vWant := reflect.ValueOf(*want)
	typeOfS := vGot.Type()

	for i := 0; i < vGot.NumField(); i++ {
		fieldGot := vGot.Field(i).Interface()
		fieldWant := vWant.Field(i).Interface()

		if !reflect.DeepEqual(fieldGot, fieldWant) {
			diffs = append(diffs, fmt.Sprintf("%s: got %v, want %v", typeOfS.Field(i).Name, fieldGot, fieldWant))
		}
	}

	if len(diffs) == 0 {
		return ""
	}

	return "Differences found:\n" + fmt.Sprint(diffs)
}

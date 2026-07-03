package cmd

import (
	"errors"
	"testing"
)

var errTestOther = errors.New("boom")

// Regression test: a failure identifying one file in a multi-file batch
// (including a bad answer to the interactive disambiguation prompt) must not
// abort the rest of the batch. batchIdentifyError should only report failure
// when every file failed.
func TestBatchIdentifyError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		errs    []error
		wantErr bool
	}{
		{name: "no files", errs: nil, wantErr: false},
		{name: "all succeeded", errs: []error{nil, nil}, wantErr: false},
		{name: "one failure among successes is not fatal", errs: []error{nil, errTestOther, nil}, wantErr: false},
		{name: "all failed", errs: []error{errTestOther, errSearch}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := batchIdentifyError(tt.errs)
			if (err != nil) != tt.wantErr {
				t.Errorf("batchIdentifyError(%v) = %v, wantErr %v", tt.errs, err, tt.wantErr)
			}
		})
	}
}

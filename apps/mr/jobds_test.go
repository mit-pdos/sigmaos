package mr

import "testing"

func TestInputDatasetRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		input string
		ds    string
		ok    bool
	}{
		{"name/ux/~local/input-data/wiki-4G/", "wiki-4G", true},
		{"name/ux/~local/input-data/wiki-20G", "wiki-20G", true},
		{"name/ux/~local/wiki-4G/", "", false},
		{"name/s3/~local/9ps3/wiki-2G/", "", false},
		{"name/ux/~local/input-data/", "", false},
	} {
		ds, ok := (&Job{Input: tc.input}).InputDataset()
		if ok != tc.ok || ds != tc.ds {
			t.Errorf("%q: got (%q,%t) want (%q,%t)", tc.input, ds, ok, tc.ds, tc.ok)
		}
	}
}

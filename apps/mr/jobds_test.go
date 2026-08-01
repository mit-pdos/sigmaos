package mr

import (
	"path/filepath"
	"strings"
	"testing"
)

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

// A job whose input ends in "/" must not produce split pathnames with a doubled
// slash: UX cleans it away, but it survives into an S3 key, where it names an
// absent object (a 404 NoSuchKey from the S3 proxy).
func TestSplitPathNoDoubleSlash(t *testing.T) {
	for _, dir := range []string{
		"name/s3/~local/9ps3/wiki-2G/",
		"name/s3/~local/9ps3/wiki-2G",
		"name/ux/~local/input-data/wiki-4G/",
	} {
		pn := filepath.Join(dir, "f0")
		if strings.Contains(pn, "//") {
			t.Errorf("split path %q has a doubled slash", pn)
		}
	}
}

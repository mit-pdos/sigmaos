package remote

import (
	"fmt"
	"path"
	"strconv"
	"strings"
)

// Corral example apps we know how to run. Their directory name under
// corral/examples is also their Makefile target and binary name.
const (
	CorralWordCount = "word_count"
	CorralGrep      = "grep"
)

// CorralConfig describes a run of the Corral baseline: which example app, on
// what input, with what task-granularity tuning.
//
// The tuning fields correspond to the flags corral.TuningFlags registers (see
// corral/flags.go). Each corral app also compiles in its own default for every
// knob (corral.SetTuningDefaults), so a zero field here means "don't pass the
// flag", leaving that default in place.
type CorralConfig struct {
	App    string // CorralWordCount or CorralGrep
	Branch string // corral branch to check out
	Bucket string // S3 bucket holding the input and receiving the output
	Input  string // input prefix within the bucket, e.g. "wiki-2G/"
	Output string // output prefix within the bucket
	Lambda bool   // run on the Lambda backend rather than locally

	// Tuning; sizes are in bytes. Zero means the app's compiled-in default.
	SplitSize      int64 // input bytes one map task reads
	MapBinSize     int64 // input splits are packed into bins this size, one per mapper
	ReduceBinSize  int64 // sets how many reducers the job has
	MaxConcurrency int   // concurrently executing mappers or reducers
	MaxLineLength  int   // maximum input line length
}

// NewCorralConfig builds a corral run configuration. Every option is a
// parameter, so that a caller's configuration is visible at the call site
// rather than buried in a default here; the caller passes the corral app's own
// tuning to run the app as it is compiled, or a different value to sweep a knob.
// A zero tuning value leaves the app's compiled-in default in place.
func NewCorralConfig(
	app, branch, bucket, input, output string,
	lambda bool,
	splitSize, mapBinSize, reduceBinSize int64,
	maxConcurrency, maxLineLength int,
) (*CorralConfig, error) {
	if app != CorralWordCount && app != CorralGrep {
		return nil, fmt.Errorf("NewCorralConfig: unknown corral app %q (want %q or %q)", app, CorralWordCount, CorralGrep)
	}
	if branch == "" {
		return nil, fmt.Errorf("NewCorralConfig %v: no branch", app)
	}
	if bucket == "" || input == "" || output == "" {
		return nil, fmt.Errorf("NewCorralConfig %v: bucket %q input %q output %q must all be set", app, bucket, input, output)
	}
	return &CorralConfig{
		App:            app,
		Branch:         branch,
		Bucket:         bucket,
		Input:          input,
		Output:         output,
		Lambda:         lambda,
		SplitSize:      splitSize,
		MapBinSize:     mapBinSize,
		ReduceBinSize:  reduceBinSize,
		MaxConcurrency: maxConcurrency,
		MaxLineLength:  maxLineLength,
	}, nil
}

func (cfg *CorralConfig) String() string {
	return fmt.Sprintf("&{ App:%v Branch:%v Bucket:%v Input:%v Output:%v Lambda:%v Tuning:%q }",
		cfg.App, cfg.Branch, cfg.Bucket, cfg.Input, cfg.Output, cfg.Lambda, cfg.tuningFlags())
}

// tuningFlags renders the flags for the knobs this config sets, and only those:
// an unset knob is left to the corral app's compiled-in default.
func (cfg *CorralConfig) tuningFlags() string {
	flags := make([]string, 0, 5)
	if cfg.SplitSize > 0 {
		flags = append(flags, "--splitsize "+strconv.FormatInt(cfg.SplitSize, 10))
	}
	if cfg.MapBinSize > 0 {
		flags = append(flags, "--mapbinsize "+strconv.FormatInt(cfg.MapBinSize, 10))
	}
	if cfg.ReduceBinSize > 0 {
		flags = append(flags, "--reducebinsize "+strconv.FormatInt(cfg.ReduceBinSize, 10))
	}
	if cfg.MaxConcurrency > 0 {
		flags = append(flags, "--maxconcurrency "+strconv.Itoa(cfg.MaxConcurrency))
	}
	if cfg.MaxLineLength > 0 {
		flags = append(flags, "--maxlinelength "+strconv.Itoa(cfg.MaxLineLength))
	}
	return strings.Join(flags, " ")
}

// inputURL is the S3 URL of the job's input.
func (cfg *CorralConfig) inputURL() string {
	return fmt.Sprintf("s3://%s/%s", cfg.Bucket, cfg.Input)
}

// inputLabel names this run's input for a results directory: the size suffix of
// the dataset when it has one ("wiki-10G/" -> "10G", matching the corral-<size>
// naming the results have used), and the whole dataset name when it doesn't
// ("gutenberg/" -> "gutenberg"). Derived from the input rather than written out
// separately, so that changing the input can't leave a run's results filed
// under the previous one's size.
func (cfg *CorralConfig) inputLabel() string {
	ds := path.Base(strings.Trim(cfg.Input, "/"))
	if i := strings.LastIndex(ds, "-"); i >= 0 && i < len(ds)-1 {
		return ds[i+1:]
	}
	return ds
}

// outputURL is the S3 URL the job's output (and its intermediate data) goes to.
func (cfg *CorralConfig) outputURL() string {
	return fmt.Sprintf("s3://%s/%s", cfg.Bucket, cfg.Output)
}

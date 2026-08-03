package remote

import (
	"slices"
	"strings"
	"testing"
)

// The configuration the corral tests build on: word_count's own compiled-in
// tuning, which is what TestCorral passes.
const (
	tstBranch    = "play-perf-asynch"
	tstBucket    = "9ps3"
	tstInput     = "wiki-2G/"
	tstOutput    = "output"
	tstSplit     = 10 * 1024 * 1024
	tstMapBin    = 130 * 1024 * 1024
	tstRedBin    = 160 * 1024 * 1024 * 5
	tstMaxConc   = 32
	tstLineLen   = 2 * 1024 * 1024
	tstLambdaMem = 1769
	tstRedGets   = 16
)

func tstCorralConfig(t *testing.T, app string, lambda bool) *CorralConfig {
	t.Helper()
	cfg, err := NewCorralConfig(app, tstBranch, tstBucket, tstInput, tstOutput, lambda,
		tstSplit, tstMapBin, tstRedBin, tstMaxConc, tstLineLen, tstLambdaMem, tstRedGets)
	if err != nil {
		t.Fatalf("NewCorralConfig(%q): %v", app, err)
	}
	return cfg
}

func TestNewCorralConfig(t *testing.T) {
	for _, app := range []string{CorralWordCount, CorralGrep} {
		cfg := tstCorralConfig(t, app, true)
		if cfg.App != app || cfg.Branch != tstBranch || cfg.Input != tstInput {
			t.Errorf("%q: unexpected config %v", app, cfg)
		}
	}
	// An unsupported app, and each required string, is rejected rather than
	// producing a command that fails on the cluster.
	if _, err := NewCorralConfig("amplab1", tstBranch, tstBucket, tstInput, tstOutput, true, 0, 0, 0, 0, 0, 0, tstRedGets); err == nil {
		t.Errorf("expected an error for an unsupported corral app")
	}
	if _, err := NewCorralConfig(CorralGrep, "", tstBucket, tstInput, tstOutput, true, 0, 0, 0, 0, 0, 0, tstRedGets); err == nil {
		t.Errorf("expected an error for an empty branch")
	}
	if _, err := NewCorralConfig(CorralGrep, tstBranch, tstBucket, "", tstOutput, true, 0, 0, 0, 0, 0, 0, tstRedGets); err == nil {
		t.Errorf("expected an error for an empty input")
	}
}

func TestCorralTuningFlags(t *testing.T) {
	cfg := tstCorralConfig(t, CorralWordCount, true)
	want := "--splitsize 10485760 --mapbinsize 136314880 --reducebinsize 838860800 --maxconcurrency 32 --maxlinelength 2097152 --lambdamemory 1769 --reducegetsconcurrency 16"
	if got := cfg.tuningFlags(); got != want {
		t.Errorf("tuningFlags = %q want %q", got, want)
	}
	// A zero knob is omitted, leaving the corral app's compiled-in default.
	cfg.MapBinSize = 0
	cfg.MaxConcurrency = 0
	want = "--splitsize 10485760 --reducebinsize 838860800 --maxlinelength 2097152 --lambdamemory 1769 --reducegetsconcurrency 16"
	if got := cfg.tuningFlags(); got != want {
		t.Errorf("tuningFlags with unset knobs = %q want %q", got, want)
	}
}

// The command must invoke the app's binary directly (not a Makefile test_*
// target), so that the input and tuning come from the config.
func TestCorralCmd(t *testing.T) {
	cfg := tstCorralConfig(t, CorralWordCount, true)
	cmd := GetCorralCmdConstructor(cfg)(&BenchConfig{}, &ClusterConfig{})
	for _, want := range []string{
		"git checkout " + tstBranch + ";",
		"cd examples/word_count;",
		"make word_count;",
		"aws s3 rm --profile sigmaos --recursive s3://9ps3/output",
		"./bin/word_count --lambda --out s3://9ps3/output " +
			"--splitsize 10485760 --mapbinsize 136314880 --reducebinsize 838860800 " +
			"--maxconcurrency 32 --maxlinelength 2097152 --lambdamemory 1769 " +
			"--reducegetsconcurrency 16 s3://9ps3/wiki-2G/",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("command missing %q:\n%s", want, cmd)
		}
	}
	// The Makefile's test_* targets hardcode the input and tuning, which is
	// what invoking the binary directly replaces.
	if strings.Contains(cmd, "test_wc_lambda") {
		t.Errorf("command still goes through the Makefile test target:\n%s", cmd)
	}
}

// A local (non-Lambda) run drops the flag rather than passing an empty one.
func TestCorralCmdLocal(t *testing.T) {
	cfg := tstCorralConfig(t, CorralGrep, false)
	cmd := GetCorralCmdConstructor(cfg)(&BenchConfig{}, &ClusterConfig{})
	// Token-exact: "--lambdamemory" contains "--lambda" as a substring.
	if slices.Contains(strings.Fields(cmd), "--lambda") {
		t.Errorf("local run passes --lambda:\n%s", cmd)
	}
	// And the memory flag, which only sizes a deployed function, is omitted too.
	if strings.Contains(cmd, "--lambdamemory") {
		t.Errorf("local run passes --lambdamemory:\n%s", cmd)
	}
	// The reduce-gets knob changes what the reduce phase measures, so it is
	// recorded in the command whichever way it is set.
	if !strings.Contains(cmd, "--reducegetsconcurrency 16") {
		t.Errorf("local run omits --reducegetsconcurrency:\n%s", cmd)
	}
	if !strings.Contains(cmd, "./bin/grep  --out") {
		t.Errorf("unexpected local invocation:\n%s", cmd)
	}
}

// The results directory name is derived from the input, so that changing the
// input can't leave a run filed under the previous input's size.
func TestCorralInputLabel(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{"wiki-10G/", "10G"},
		{"wiki-2G", "2G"},
		{"wiki-1.8G/", "1.8G"},
		{"wiki-128M/", "128M"},
		// No size suffix: the whole dataset name is the label.
		{"gutenberg/", "gutenberg"},
		// A nested prefix labels by its last element.
		{"data/wiki-20G/", "20G"},
		// A trailing dash has no size after it, so fall back to the name.
		{"wiki-/", "wiki-"},
	} {
		cfg := &CorralConfig{Input: tc.input}
		if got := cfg.inputLabel(); got != tc.want {
			t.Errorf("inputLabel(%q) = %q want %q", tc.input, got, tc.want)
		}
	}
}

// The results directory carries the app and the dataset, so that runs of
// different workloads on different dataset sizes can't collide, and so that a
// corral run files alongside the σOS MR run on the same data. The graph script
// is pointed at these names, so they are worth pinning down.
func TestCorralResultsDirName(t *testing.T) {
	for _, tc := range []struct {
		app   string
		input string
		start string
		want  string
	}{
		{CorralWordCount, "wiki-10G/", "warm", "corral-wc-wiki10G-warm"},
		{CorralWordCount, "wiki-10G/", "cold", "corral-wc-wiki10G-cold"},
		{CorralGrep, "wiki-2G/", "warm", "corral-grep-wiki2G-warm"},
		{CorralGrep, "wiki-128M", "warm", "corral-grep-wiki128M-warm"},
		// A nested prefix is named by its last element, as with inputLabel.
		{CorralWordCount, "data/wiki-20G/", "warm", "corral-wc-wiki20G-warm"},
		// A dataset with no size suffix keeps its whole name.
		{CorralGrep, "gutenberg/", "warm", "corral-grep-gutenberg-warm"},
	} {
		cfg := &CorralConfig{App: tc.app, Input: tc.input}
		if got := cfg.ResultsDirName(tc.start); got != tc.want {
			t.Errorf("ResultsDirName(%q, %q, %q) = %q want %q", tc.app, tc.input, tc.start, got, tc.want)
		}
	}
}

package remote

import (
	"strings"
	"testing"
)

// The configuration the corral tests build on: word_count's own compiled-in
// tuning, which is what TestCorral passes.
const (
	tstBranch  = "play-perf-asynch"
	tstBucket  = "9ps3"
	tstInput   = "wiki-2G/"
	tstOutput  = "output"
	tstSplit   = 10 * 1024 * 1024
	tstMapBin  = 130 * 1024 * 1024
	tstRedBin  = 160 * 1024 * 1024 * 5
	tstMaxConc = 32
	tstLineLen = 2 * 1024 * 1024
)

func tstCorralConfig(t *testing.T, app string, lambda bool) *CorralConfig {
	t.Helper()
	cfg, err := NewCorralConfig(app, tstBranch, tstBucket, tstInput, tstOutput, lambda,
		tstSplit, tstMapBin, tstRedBin, tstMaxConc, tstLineLen)
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
	if _, err := NewCorralConfig("amplab1", tstBranch, tstBucket, tstInput, tstOutput, true, 0, 0, 0, 0, 0); err == nil {
		t.Errorf("expected an error for an unsupported corral app")
	}
	if _, err := NewCorralConfig(CorralGrep, "", tstBucket, tstInput, tstOutput, true, 0, 0, 0, 0, 0); err == nil {
		t.Errorf("expected an error for an empty branch")
	}
	if _, err := NewCorralConfig(CorralGrep, tstBranch, tstBucket, "", tstOutput, true, 0, 0, 0, 0, 0); err == nil {
		t.Errorf("expected an error for an empty input")
	}
}

func TestCorralTuningFlags(t *testing.T) {
	cfg := tstCorralConfig(t, CorralWordCount, true)
	want := "--splitsize 10485760 --mapbinsize 136314880 --reducebinsize 838860800 --maxconcurrency 32 --maxlinelength 2097152"
	if got := cfg.tuningFlags(); got != want {
		t.Errorf("tuningFlags = %q want %q", got, want)
	}
	// A zero knob is omitted, leaving the corral app's compiled-in default.
	cfg.MapBinSize = 0
	cfg.MaxConcurrency = 0
	want = "--splitsize 10485760 --reducebinsize 838860800 --maxlinelength 2097152"
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
			"--maxconcurrency 32 --maxlinelength 2097152 s3://9ps3/wiki-2G/",
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
	if strings.Contains(cmd, "--lambda") {
		t.Errorf("local run passes --lambda:\n%s", cmd)
	}
	if !strings.Contains(cmd, "./bin/grep  --out") {
		t.Errorf("unexpected local invocation:\n%s", cmd)
	}
}

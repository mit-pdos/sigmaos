package benchmarks_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"sigmaos/benchmarks"
	"sigmaos/proc"
	"sigmaos/test"
)

type zygoteWorkload struct {
	name   string
	script string
}

func getZygoteWorkload(name string) (zygoteWorkload, error) {
	switch name {
	case "hello":
		return zygoteWorkload{name: name, script: "benchmarks/hello/main.py"}, nil
	case "numpy_pandas":
		return zygoteWorkload{name: name, script: "benchmarks/numpy_pandas/main.py"}, nil
	case "pytorch":
		return zygoteWorkload{name: name, script: "benchmarks/pytorch/main.py"}, nil
	case "memory":
		return zygoteWorkload{name: name, script: "benchmarks/memory/memory.py"}, nil
	default:
		return zygoteWorkload{}, fmt.Errorf("unknown zygote workload %q", name)
	}
}

func buildPythonProc(w zygoteWorkload, hold time.Duration) *proc.Proc {
	p := proc.NewPythonProc(proc.Python311, []string{w.script})
	if hold > 0 {
		p.AppendEnv("ZYGOTE_BENCH_HOLD_SECS", strconv.FormatFloat(hold.Seconds(), 'f', 3, 64))
	}
	return p
}

func buildForkProc(cfg proc.ForkConfig, childName string, hold time.Duration) *proc.Proc {
	p := proc.NewForkProc(cfg, []string{childName})
	if hold > 0 {
		p.AppendEnv("ZYGOTE_BENCH_HOLD_SECS", strconv.FormatFloat(hold.Seconds(), 'f', 3, 64))
	}
	return p
}

func spawnAndWaitRound(ts *test.Tstate, w zygoteWorkload, n int, useFork bool, hold time.Duration, cfg proc.ForkConfig, nZygotes int) (time.Duration, error) {
	var wg sync.WaitGroup
	errCh := make(chan error, n)

	var forkCfgs []proc.ForkConfig
	if useFork {
		// Add a random environment variable to ensure that we don't reuse
		// zygotes across trials.
		uniqueId := fmt.Sprintf("%d", time.Now().UnixNano())

		// TODO: Remove this experimental multi-zygote code...
		for i := 0; i < nZygotes; i++ {
			copy := proc.ForkConfig{
				ZygoteProc: cfg.ZygoteProc.Clone(),
				KeepAlive:  cfg.KeepAlive,
			}
			copy.ZygoteProc.AppendEnv("__ZYGOTE_BENCHMARK", uniqueId+"-"+strconv.Itoa(i))
			forkCfgs = append(forkCfgs, copy)
		}
	}

	start := time.Now()
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var p *proc.Proc
			if useFork {
				cfg := forkCfgs[i%nZygotes]
				p = buildForkProc(cfg, "child-"+strconv.Itoa(i), hold)
			} else {
				p = buildPythonProc(w, hold)
			}
			if p == nil {
				errCh <- fmt.Errorf("failed to build proc")
				return
			}
			if err := ts.Spawn(p); err != nil {
				errCh <- fmt.Errorf("spawn[%d]: %w", i, err)
				return
			}
			if err := ts.WaitStart(p.GetPid()); err != nil {
				errCh <- fmt.Errorf("waitstart[%d]: %w", i, err)
				return
			}
			st, err := ts.WaitExit(p.GetPid())
			if err != nil {
				errCh <- fmt.Errorf("waitexit[%d]: %w", i, err)
				return
			}
			if !st.IsStatusOK() {
				errCh <- fmt.Errorf("bad status[%d]: %v", i, st)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return 0, err
		}
	}
	return time.Since(start), nil
}

func printZygoteStats(label string, rs *benchmarks.Results) {
	mean, _ := rs.Mean()
	std, _ := rs.StdDev()
	p50, _ := rs.Percentile(50)
	p90, _ := rs.Percentile(90)
	p99, _ := rs.Percentile(99)
	p999, _ := rs.Percentile(99.9)
	fmt.Printf("%s mean=%v std=%v p50=%v p90=%v p99=%v p99.9=%v\n", label, mean, std, p50, p90, p99, p999)
}

func TestZygoteForkComparison(t *testing.T) {
	if ZYGOTE_NPROCS <= 0 {
		t.Fatalf("zygote_nprocs must be > 0")
	}
	if ZYGOTE_NTRIALS <= 0 {
		t.Fatalf("zygote_ntrials must be > 0")
	}

	w, err := getZygoteWorkload(ZYGOTE_WORKLOAD)
	if err != nil {
		t.Fatal(err)
	}

	ts, err := test.NewTstateAll(t)
	if err != nil {
		t.Fatalf("new tstate: %v", err)
	}
	defer ts.Shutdown()

	zygoteProc := buildPythonProc(w, 0)
	forkCfg := proc.ForkConfig{ZygoteProc: zygoteProc, KeepAlive: ZYGOTE_KEEPALIVE}

	if _, err := spawnAndWaitRound(ts, w, 1, false, 0, forkCfg, 1); err != nil {
		t.Fatalf("warmup: %v", err)
	}

	baselineRound := benchmarks.NewResults(ZYGOTE_NTRIALS, benchmarks.OPS)
	baselinePerProc := benchmarks.NewResults(ZYGOTE_NTRIALS, benchmarks.OPS)
	for i := 0; i < ZYGOTE_NTRIALS; i++ {
		d, err := spawnAndWaitRound(ts, w, ZYGOTE_NPROCS, false, 0, forkCfg, 1)
		if err != nil {
			t.Fatalf("baseline trial %d: %v", i, err)
		}
		baselineRound.Append(d, float64(ZYGOTE_NPROCS))
		baselinePerProc.Append(d/time.Duration(ZYGOTE_NPROCS), 1)
	}

	forkRound := benchmarks.NewResults(ZYGOTE_NTRIALS, benchmarks.OPS)
	forkPerProc := benchmarks.NewResults(ZYGOTE_NTRIALS, benchmarks.OPS)
	for i := 0; i < ZYGOTE_NTRIALS; i++ {
		// TODO: In the future we should fork the root zygote to reduce memory overhead.
		nZygotes := min(runtime.NumCPU(), ZYGOTE_NPROCS/32)
		d, err := spawnAndWaitRound(ts, w, ZYGOTE_NPROCS, true, 0, forkCfg, nZygotes)
		if err != nil {
			t.Fatalf("fork trial %d: %v", i, err)
		}
		forkRound.Append(d, float64(ZYGOTE_NPROCS))
		forkPerProc.Append(d/time.Duration(ZYGOTE_NPROCS), 1)
	}

	bMean, _ := baselineRound.Mean()
	fMean, _ := forkRound.Mean()
	bP99, _ := baselineRound.Percentile(99)
	fP99, _ := forkRound.Percentile(99)

	fmt.Printf("\n=== Zygote Fork Comparison (%s) ===\n", w.name)
	fmt.Printf("trials=%d nprocs=%d keepalive=%v\n", ZYGOTE_NTRIALS, ZYGOTE_NPROCS, ZYGOTE_KEEPALIVE)
	printZygoteStats("baseline_round", baselineRound)
	printZygoteStats("fork_round", forkRound)
	printZygoteStats("baseline_per_proc", baselinePerProc)
	printZygoteStats("fork_per_proc", forkPerProc)
	fmt.Printf("speedup_mean=%.2fx speedup_p99=%.2fx\n", float64(bMean)/float64(fMean), float64(bP99)/float64(fP99))
}

func parseLevels(s string) ([]int, error) {
	parts := strings.Split(s, ",")
	levels := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("bad level %q: %w", p, err)
		}
		if n <= 0 {
			return nil, fmt.Errorf("memory level must be > 0: %d", n)
		}
		levels = append(levels, n)
	}
	if len(levels) == 0 {
		return nil, fmt.Errorf("no memory levels configured")
	}
	sort.Ints(levels)
	return levels, nil
}

func debugSelectorsSet(s string) map[string]bool {
	out := make(map[string]bool)
	for _, p := range strings.Split(s, ";") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out[p] = true
	}
	return out
}

func ensurePSSDebugEnabled(t *testing.T) func() {
	t.Helper()
	prev, hadPrev := os.LookupEnv("SIGMADEBUG")
	sel := debugSelectorsSet(prev)
	sel["PSS"] = true
	sel["PSS_ERR"] = true

	keys := make([]string, 0, len(sel))
	for k := range sel {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	updated := strings.Join(keys, ";")
	if updated != "" {
		updated += ";"
	}
	if err := os.Setenv("SIGMADEBUG", updated); err != nil {
		t.Fatalf("set SIGMADEBUG: %v", err)
	}
	return func() {
		if hadPrev {
			_ = os.Setenv("SIGMADEBUG", prev)
		} else {
			_ = os.Unsetenv("SIGMADEBUG")
		}
	}
}

var pssLineRE = regexp.MustCompile(`\[([^\]]+)\]\s+PSS:\s+([0-9]+)KB`)

func runLogsScript() (string, error) {
	for _, path := range []string{"./logs.sh", "../logs.sh"} {
		cmd := exec.Command("bash", path, "--merge")
		out, err := cmd.CombinedOutput()
		if err == nil {
			return string(out), nil
		}
		if !bytes.Contains(out, []byte("No such file or directory")) {
			return "", fmt.Errorf("run %s: %w (%s)", path, err, strings.TrimSpace(string(out)))
		}
	}
	return "", fmt.Errorf("could not find logs.sh in ./ or ../")
}

func parsePSSFromLogs(logs string, target map[string]bool) map[string]uint64 {
	out := make(map[string]uint64)
	for _, m := range pssLineRE.FindAllStringSubmatch(logs, -1) {
		if len(m) != 3 {
			continue
		}
		pid := m[1]
		if !target[pid] {
			continue
		}
		kb, err := strconv.ParseUint(m[2], 10, 64)
		if err != nil {
			continue
		}
		out[pid] = kb
	}
	return out
}

func collectPSSForPids(pids []string, wait time.Duration) (map[string]uint64, error) {
	target := make(map[string]bool, len(pids))
	for _, pid := range pids {
		target[pid] = true
	}

	deadline := time.Now().Add(wait)
	for {
		logs, err := runLogsScript()
		if err != nil {
			return nil, err
		}
		pss := parsePSSFromLogs(logs, target)
		if len(pss) == len(target) {
			return pss, nil
		}
		if time.Now().After(deadline) {
			missing := make([]string, 0)
			for pid := range target {
				if _, ok := pss[pid]; !ok {
					missing = append(missing, pid)
				}
			}
			sort.Strings(missing)
			return nil, fmt.Errorf("missing PSS samples for %d/%d pids: %v", len(missing), len(target), missing)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func runMemoryScenario(ts *test.Tstate, w zygoteWorkload, n int, useFork bool, cfg proc.ForkConfig, pssDelay time.Duration) (uint64, error) {
	procs := make([]*proc.Proc, 0, n)
	pssDelayMS := int(pssDelay.Milliseconds())
	if pssDelayMS <= 0 {
		pssDelayMS = 1
	}

	for i := 0; i < n; i++ {
		var p *proc.Proc
		if useFork {
			p = buildForkProc(cfg, fmt.Sprintf("mem-child-%d", i), ZYGOTE_MEM_HOLD)
		} else {
			p = buildPythonProc(w, ZYGOTE_MEM_HOLD)
		}
		if p == nil {
			return 0, fmt.Errorf("failed to build proc")
		}
		p.SetMeasurePSS(true, pssDelayMS)
		procs = append(procs, p)
	}

	for i, p := range procs {
		if err := ts.Spawn(p); err != nil {
			return 0, fmt.Errorf("spawn[%d]: %w", i, err)
		}
	}

	for i, p := range procs {
		if err := ts.WaitStart(p.GetPid()); err != nil {
			return 0, fmt.Errorf("waitstart[%d]: %w", i, err)
		}
	}

	pids := make([]string, 0, len(procs))
	for _, p := range procs {
		pids = append(pids, p.GetPid().String())
	}

	for _, p := range procs {
		st, err := ts.WaitExit(p.GetPid())
		if err != nil {
			return 0, fmt.Errorf("waitexit[%s]: %w", p.GetPid(), err)
		}
		if !st.IsStatusOK() {
			return 0, fmt.Errorf("bad status[%s]: %v", p.GetPid(), st)
		}
	}

	pssByPid, err := collectPSSForPids(pids, 2*time.Second)
	if err != nil {
		return 0, err
	}

	totalKB := uint64(0)
	for _, pid := range pids {
		totalKB += pssByPid[pid]
	}
	return totalKB, nil
}

func TestZygoteForkMemoryScaling(t *testing.T) {
	restoreDebug := ensurePSSDebugEnabled(t)
	defer restoreDebug()

	levels, err := parseLevels(ZYGOTE_MEM_LEVELS)
	if err != nil {
		t.Fatal(err)
	}

	w, err := getZygoteWorkload(ZYGOTE_WORKLOAD)
	if err != nil {
		t.Fatal(err)
	}

	ts, err := test.NewTstateAll(t)
	if err != nil {
		t.Fatalf("new tstate: %v", err)
	}
	defer ts.Shutdown()

	zygoteProc := buildPythonProc(w, 0)
	forkCfg := proc.ForkConfig{ZygoteProc: zygoteProc, KeepAlive: ZYGOTE_KEEPALIVE}

	if _, err := spawnAndWaitRound(ts, w, 1, false, 0, forkCfg, 1); err != nil {
		t.Fatalf("warmup: %v", err)
	}
	if ZYGOTE_MEM_HOLD <= ZYGOTE_PSS_DELAY+500*time.Millisecond {
		t.Fatalf("zygote_mem_hold (%v) must be > zygote_pss_delay (%v) + 500ms", ZYGOTE_MEM_HOLD, ZYGOTE_PSS_DELAY)
	}

	fmt.Printf("\n=== Zygote Memory Scaling (%s) ===\n", w.name)
	fmt.Printf("levels=%v hold=%v pss_delay=%v\n", levels, ZYGOTE_MEM_HOLD, ZYGOTE_PSS_DELAY)
	for _, n := range levels {
		baselineUsedKB, err := runMemoryScenario(ts, w, n, false, forkCfg, ZYGOTE_PSS_DELAY)
		if err != nil {
			t.Fatalf("baseline n=%d: %v", n, err)
		}
		forkUsedKB, err := runMemoryScenario(ts, w, n, true, forkCfg, ZYGOTE_PSS_DELAY)
		if err != nil {
			t.Fatalf("fork n=%d: %v", n, err)
		}

		forkRatio := 0.0
		if baselineUsedKB > 0 {
			forkRatio = float64(forkUsedKB) / float64(baselineUsedKB)
		}
		fmt.Printf("n=%d baseline_pss_kb=%d baseline_pss_mb=%.2f fork_pss_kb=%d fork_pss_mb=%.2f fork_ratio=%.3f\n",
			n,
			baselineUsedKB,
			float64(baselineUsedKB)/1024.0,
			forkUsedKB,
			float64(forkUsedKB)/1024.0,
			forkRatio,
		)
	}
}

func spawnAndWait(ts *test.Tstate, p *proc.Proc) error {
	if err := ts.Spawn(p); err != nil {
		return fmt.Errorf("spawn: %w", err)
	}
	if err := ts.WaitStart(p.GetPid()); err != nil {
		return fmt.Errorf("waitstart: %w", err)
	}
	st, err := ts.WaitExit(p.GetPid())
	if err != nil {
		return fmt.Errorf("waitexit: %w", err)
	}
	if !st.IsStatusOK() {
		return fmt.Errorf("bad status: %v", st)
	}
	return nil
}

func parseStartLatencies(logs string) ([]time.Duration, []time.Duration, error) {
	var spawnLatencies []time.Duration
	var forkLatencies []time.Duration

	re := regexp.MustCompile(`sinceSpawn:(\d+)us`)

	lines := strings.Split(logs, "\n")
	for _, line := range lines {
		if strings.Contains(line, "E2e spawn time since spawn until main") {
			m := re.FindStringSubmatch(line)
			if m == nil {
				return nil, nil, fmt.Errorf("failed to parse spawn latency from line: %s", line)
			}

			val, err := strconv.Atoi(m[1])
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse spawn latency from line: %s", line)
			}

			spawnLatencies = append(spawnLatencies, time.Duration(val)*time.Microsecond)
		} else if strings.Contains(line, "E2e spawn time since fork until main") {
			m := re.FindStringSubmatch(line)
			if m == nil {
				return nil, nil, fmt.Errorf("failed to parse fork latency from line: %s", line)
			}

			val, err := strconv.Atoi(m[1])
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse fork latency from line: %s", line)
			}

			forkLatencies = append(forkLatencies, time.Duration(val)*time.Microsecond)
		}
	}

	return spawnLatencies, forkLatencies, nil
}

func TestPythonStartLatency(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	const N = 100

	// Without forking
	createProc := func() *proc.Proc {
		return proc.NewPythonProc(proc.Python311, []string{"benchmarks/startup_latency/main.py"})
	}

	for i := 0; i <= N; i++ {
		spawnAndWait(ts, createProc())
	}

	// With forking
	forkConfig := proc.ForkConfig{
		ZygoteProc: createProc(),
		KeepAlive:  10 * time.Second,
	}

	for i := 0; i <= N; i++ {
		spawnAndWait(ts, proc.NewForkProc(forkConfig, []string{}))
	}

	// Collect logs and parse latencies
	logs, err := runLogsScript()
	if err != nil {
		t.Fatalf("collect logs: %v", err)
	}

	spawnLatencies, forkLatencies, err := parseStartLatencies(logs)
	if err != nil {
		t.Fatalf("parse latencies: %v", err)
	}

	spawnLatencies = spawnLatencies[1:] // Skip the first spawn which is a warmup
	forkLatencies = forkLatencies[1:]   // Skip the first fork which is a warmup

	spawnResults := benchmarks.NewResults(N, benchmarks.OPS)
	forkResults := benchmarks.NewResults(N, benchmarks.OPS)

	for _, lat := range spawnLatencies {
		spawnResults.Append(lat, 1)
	}
	for _, lat := range forkLatencies {
		forkResults.Append(lat, 1)
	}

	latSpawn, _ := spawnResults.Summary()
	latFork, _ := forkResults.Summary()

	fmt.Printf("\n=== Python Start Latencies ===\n")
	fmt.Printf("Spawn latencies (without fork):%v\n\n", latSpawn)
	fmt.Printf("Fork latencies (with fork):%v\n", latFork)
}

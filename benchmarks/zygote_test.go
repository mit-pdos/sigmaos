package benchmarks_test

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"sigmaos/benchmarks"
	"sigmaos/proc"
	"sigmaos/test"

	"github.com/stretchr/testify/assert"
)

type zygoteWorkload struct {
	name   string
	script string
	args   []string
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
	case "random_forest":
		return zygoteWorkload{name: name, script: "benchmarks/random_forest/main.py"}, nil
	case "imgresize":
		return zygoteWorkload{name: name, script: "benchmarks/imgresize/main.py", args: []string{"name/s3/~any/9ps3/img-save/1.jpg", "name/ux/~local/"}}, nil
	default:
		return zygoteWorkload{}, fmt.Errorf("unknown zygote workload %q", name)
	}
}

func buildPythonProc(w zygoteWorkload, hold time.Duration) *proc.Proc {
	args := []string{w.script}
	if (w.args) != nil {
		args = append(args, w.args...)
	}

	p := proc.NewPythonProc(proc.Python311, args)
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

func spawnAndWaitRound(ts *test.Tstate, w zygoteWorkload, n int, useFork bool, hold time.Duration, cfg proc.ForkConfig, nZygotes int, concurrency int, warmZygotes bool) (time.Duration, error) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	errCh := make(chan error, n)
	nZygotes = max(1, min(n, nZygotes))

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

	if useFork && warmZygotes {
		for i := 0; i < nZygotes; i++ {
			wg.Add(1)
			go func(i int) {
				p := buildForkProc(forkCfgs[i], "warmup-child", hold)
				if err := spawnAndWait(ts, p); err != nil {
					panic(fmt.Errorf("warmup: %w", err))
				}
				wg.Done()
			}(i)
		}
		wg.Wait()
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

			if concurrency > 0 {
				// Optionally limit concurrency to avoid overwhelming the system.
				sem <- struct{}{}
				defer func() { <-sem }()
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

	go func() {
		wg.Wait()
		close(errCh)
	}()

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

	if _, err := spawnAndWaitRound(ts, w, 1, false, 0, forkCfg, 1, 0, false); err != nil {
		t.Fatalf("warmup: %v", err)
	}

	baselineRound := benchmarks.NewResults(ZYGOTE_NTRIALS, benchmarks.OPS)
	baselinePerProc := benchmarks.NewResults(ZYGOTE_NTRIALS, benchmarks.OPS)
	for i := 0; i < ZYGOTE_NTRIALS; i++ {
		d, err := spawnAndWaitRound(ts, w, ZYGOTE_NPROCS, false, 0, forkCfg, 1, 512, false)
		if err != nil {
			t.Fatalf("baseline trial %d: %v", i, err)
		}
		baselineRound.Append(d, float64(ZYGOTE_NPROCS))
		baselinePerProc.Append(d/time.Duration(ZYGOTE_NPROCS), 1)
		fmt.Printf("baseline trial %d: %v\n", i, d)
	}

	forkRound := benchmarks.NewResults(ZYGOTE_NTRIALS, benchmarks.OPS)
	forkPerProc := benchmarks.NewResults(ZYGOTE_NTRIALS, benchmarks.OPS)
	for i := 0; i < ZYGOTE_NTRIALS; i++ {
		// TODO: In the future we should fork the root zygote to reduce memory overhead.
		//       This can be done by simply calling os.fork() in the zygote.
		var nZygotes = 1
		if ZYGOTE_WORKLOAD == "numpy_pandas" || ZYGOTE_WORKLOAD == "hello" {
			nZygotes = min(4, ZYGOTE_NPROCS/64)
		}

		d, err := spawnAndWaitRound(ts, w, ZYGOTE_NPROCS, true, 0, forkCfg, nZygotes, 0, false)
		if err != nil {
			t.Fatalf("fork trial %d: %v", i, err)
		}
		forkRound.Append(d, float64(ZYGOTE_NPROCS))
		forkPerProc.Append(d/time.Duration(ZYGOTE_NPROCS), 1)
		fmt.Printf("    fork trial %d: %v\n", i, d)
	}

	warmForkRound := benchmarks.NewResults(ZYGOTE_NTRIALS, benchmarks.OPS)
	warmForkPerProc := benchmarks.NewResults(ZYGOTE_NTRIALS, benchmarks.OPS)
	for i := 0; i < ZYGOTE_NTRIALS; i++ {
		var nZygotes = min(4, ZYGOTE_NPROCS)

		d, err := spawnAndWaitRound(ts, w, ZYGOTE_NPROCS, true, 0, forkCfg, nZygotes, 0, true)
		if err != nil {
			t.Fatalf("warm fork trial %d: %v", i, err)
		}
		warmForkRound.Append(d, float64(ZYGOTE_NPROCS))
		warmForkPerProc.Append(d/time.Duration(ZYGOTE_NPROCS), 1)
		fmt.Printf("wrm fork trial %d: %v\n", i, d)
	}

	bMean, _ := baselineRound.Mean()
	fMean, _ := forkRound.Mean()
	bP99, _ := baselineRound.Percentile(99)
	fP99, _ := forkRound.Percentile(99)

	fmt.Printf("\n=== Zygote Fork Comparison (%s) ===\n", w.name)
	fmt.Printf("trials=%d nprocs=%d keepalive=%v\n", ZYGOTE_NTRIALS, ZYGOTE_NPROCS, ZYGOTE_KEEPALIVE)
	printZygoteStats("baseline_round", baselineRound)
	printZygoteStats("fork_round", forkRound)
	printZygoteStats("warm_fork_round", warmForkRound)
	printZygoteStats("baseline_per_proc", baselinePerProc)
	printZygoteStats("fork_per_proc", forkPerProc)
	printZygoteStats("warm_fork_per_proc", warmForkPerProc)
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
	benchmarks.EnsureSigmaDebugEnabled(t, "PSS", "PSS_ERR")

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

	if _, err := spawnAndWaitRound(ts, w, 1, false, 0, forkCfg, 1, 0, false); err != nil {
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

type spawnLatencyLine struct {
	pid        string
	message    string
	op         time.Duration
	sinceSpawn time.Duration
}

var spawnLatRe = regexp.MustCompile(
	`\[([^\]]+)\]\s+(.*?)\s+op:([^\s]+)\s+sinceSpawn:([^\s]+)`,
)

func parseLine(s string) (*spawnLatencyLine, error) {
	m := spawnLatRe.FindStringSubmatch(s)
	if m == nil {
		return nil, fmt.Errorf("no match")
	}

	op, err := time.ParseDuration(m[3])
	if err != nil {
		return nil, fmt.Errorf("parse op: %w", err)
	}

	since, err := time.ParseDuration(m[4])
	if err != nil {
		return nil, fmt.Errorf("parse sinceSpawn: %w", err)
	}

	return &spawnLatencyLine{
		pid:        m[1],
		message:    m[2],
		op:         op,
		sinceSpawn: since,
	}, nil
}

type spawnLatencyResult struct {
	// A: All
	// S: Spawn only
	// F: Fork only

	schedulingLat     spawnLatencyLine // A: Paper.Setup.GlobalScheduling
	containerStartLat spawnLatencyLine // S: Paper.Setup.ContainerStart
	fmRequestSentLat  spawnLatencyLine // F: forkMgr.forkChild send fork request
	fpCloneLat        spawnLatencyLine // F: splib.fork.fork_point clone
	fpNotifyLat       spawnLatencyLine // F: splib.fork.fork_point notify supervisor
	fpApplyEnvLat     spawnLatencyLine // F: splib.fork.fork_point apply env
	e2eLatency        spawnLatencyLine // S: E2e spawn time  |  F:  splib.fork.fork_point done
}

func parseStartLatencies(logs string, spawnPids map[string]*spawnLatencyResult, forkPids map[string]*spawnLatencyResult) {

	lines := strings.Split(logs, "\n")
	for _, line := range lines {
		if !strings.Contains(line, " SPAWN_LAT ") {
			continue
		}

		latLine, err := parseLine(line)
		if err != nil {
			continue
		}

		if _, exists := spawnPids[latLine.pid]; exists {
			if spawnPids[latLine.pid] == nil {
				spawnPids[latLine.pid] = &spawnLatencyResult{}
			}

			switch latLine.message {
			case "Paper.Setup.GlobalScheduling":
				spawnPids[latLine.pid].schedulingLat = *latLine
			case "Paper.Setup.ContainerStart":
				spawnPids[latLine.pid].containerStartLat = *latLine
			case "E2e spawn time":
				spawnPids[latLine.pid].e2eLatency = *latLine
			}
		} else if _, exists := forkPids[latLine.pid]; exists {
			if forkPids[latLine.pid] == nil {
				forkPids[latLine.pid] = &spawnLatencyResult{}
			}

			switch latLine.message {
			case "Paper.Setup.GlobalScheduling":
				forkPids[latLine.pid].schedulingLat = *latLine
			case "forkMgr.forkChild send fork request":
				forkPids[latLine.pid].fmRequestSentLat = *latLine
			case "splib.fork.fork_point clone":
				forkPids[latLine.pid].fpCloneLat = *latLine
			case "splib.fork.fork_point notify supervisor":
				forkPids[latLine.pid].fpNotifyLat = *latLine
			case "splib.fork.fork_point apply env":
				forkPids[latLine.pid].fpApplyEnvLat = *latLine
			case "splib.fork.fork_point done":
				forkPids[latLine.pid].e2eLatency = *latLine
			}
		}
	}
}

func TestPythonStartLatency(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	const N = 1000

	spawnLats := make(map[string]*spawnLatencyResult)
	forkLats := make(map[string]*spawnLatencyResult)

	// Without forking
	createProc := func() *proc.Proc {
		return proc.NewPythonProc(proc.Python311, []string{"benchmarks/startup_latency/main.py"})
	}

	for i := 0; i <= N; i++ {
		p := createProc()
		if i > 0 {
			spawnLats[string(p.GetPid())] = nil
		}
		spawnAndWait(ts, p)
	}

	// With forking
	forkConfig := proc.ForkConfig{
		ZygoteProc: createProc(),
		KeepAlive:  10 * time.Second,
	}

	for i := 0; i <= N; i++ {
		p := proc.NewForkProc(forkConfig, []string{})
		if i > 0 {
			forkLats[string(p.GetPid())] = nil
		}
		spawnAndWait(ts, p)
	}

	// Collect logs and parse latencies
	logs, err := runLogsScript()
	if err != nil {
		t.Fatalf("collect logs: %v", err)
	}

	parseStartLatencies(logs, spawnLats, forkLats)
	if err != nil {
		t.Fatalf("parse latencies: %v", err)
	}

	for procType, lats := range map[string]map[string]*spawnLatencyResult{
		"spawn": spawnLats,
		"fork":  forkLats,
	} {
		fmt.Printf("\n=== %s latencies ===\n", procType)

		// For each field in spawnLatencyResult:
		fields := []string{
			"schedulingLat",
			"containerStartLat",
			"fmRequestSentLat",
			"fpCloneLat",
			"fpNotifyLat",
			"fpApplyEnvLat",
			"e2eLatency",
		}

		fieldGetters := map[string]func(*spawnLatencyResult) spawnLatencyLine{
			"schedulingLat":     func(r *spawnLatencyResult) spawnLatencyLine { return r.schedulingLat },
			"containerStartLat": func(r *spawnLatencyResult) spawnLatencyLine { return r.containerStartLat },
			"fmRequestSentLat":  func(r *spawnLatencyResult) spawnLatencyLine { return r.fmRequestSentLat },
			"fpCloneLat":        func(r *spawnLatencyResult) spawnLatencyLine { return r.fpCloneLat },
			"fpNotifyLat":       func(r *spawnLatencyResult) spawnLatencyLine { return r.fpNotifyLat },
			"fpApplyEnvLat":     func(r *spawnLatencyResult) spawnLatencyLine { return r.fpApplyEnvLat },
			"e2eLatency":        func(r *spawnLatencyResult) spawnLatencyLine { return r.e2eLatency },
		}

		for _, fieldName := range fields {
			getField := fieldGetters[fieldName]
			opResults := benchmarks.NewResults(N, benchmarks.OPS)
			sinceSpawnResults := benchmarks.NewResults(N, benchmarks.OPS)

			for pid, res := range lats {
				if res == nil {
					fmt.Printf("  pid=%s missing data\n", pid)
					continue
				}
				opResults.Append(getField(res).op, 1)
				sinceSpawnResults.Append(getField(res).sinceSpawn, 1)
			}

			opMean, _ := opResults.Mean()
			opStd, _ := opResults.StdDev()
			sinceSpawnMean, _ := sinceSpawnResults.Mean()
			sinceSpawnStd, _ := sinceSpawnResults.StdDev()
			fmt.Printf("  %s: op_mean=%v (std %v) since_spawn_mean=%v (std %v)\n", fieldName, opMean, opStd, sinceSpawnMean, sinceSpawnStd)
		}
	}
}

const (
	THROUGHPUT_BASELINE_SCRIPT = "benchmarks/throughput/baseline.py"
	THROUGHPUT_FORK_SCRIPT     = "benchmarks/throughput/fork.py"
)

func spawnBurstWaitExitProcs(ts *test.Tstate, procs []*proc.Proc) time.Duration {
	per := len(procs) / N_THREADS
	start := time.Now()
	done := make(chan bool)
	for i := 0; i < N_THREADS; i++ {
		go func(i int) {
			chunk := procs[i*per : (i+1)*per]
			for _, p := range chunk {
				err := ts.Spawn(p)
				assert.Nil(ts.T, err, "Error Spawn: %v", err)
			}
			for _, p := range chunk {
				ts.WaitExit(p.GetPid())
			}
			done <- true
		}(i)
	}
	for i := 0; i < N_THREADS; i++ {
		<-done
	}

	return time.Since(start)
}

func runZygoteThroughputTrial(ts *test.Tstate, useFork bool, n int, forkCfg proc.ForkConfig) time.Duration {
	procs := make([]*proc.Proc, n)

	if useFork {
		var forkCfgs []proc.ForkConfig
		for i := 0; i < N_THREADS; i++ {
			forkProc := proc.NewProc(forkCfg.ZygoteProc.GetProgram(), append([]string{}, forkCfg.ZygoteProc.Args...))
			forkProc.GetProcEnv().UseSPProxy = forkCfg.ZygoteProc.GetProcEnv().UseSPProxy
			forkProc.GetProcEnv().UseSPProxyProcClnt = forkCfg.ZygoteProc.GetProcEnv().UseSPProxyProcClnt

			for k, v := range forkCfg.ZygoteProc.Env {
				if _, ok := forkProc.Env[k]; ok {
					continue
				}
				forkProc.Env[k] = v
			}

			forkProc.AppendEnv("__ZYGOTE_BENCHMARK", fmt.Sprintf("throughput-%d", i))
			forkCfgClone := proc.ForkConfig{
				ZygoteProc: forkProc,
				KeepAlive:  forkCfg.KeepAlive,
			}
			forkCfgs = append(forkCfgs, forkCfgClone)
		}

		// Warm up zygotes
		for i := 0; i < N_THREADS; i++ {
			cfg := forkCfgs[i]
			p := proc.NewForkProc(cfg, []string{})
			ts.Spawn(p)
			ts.WaitExit(p.GetPid())
		}

		i := 0
		for j := 0; j < N_THREADS; j++ {
			cfg := forkCfgs[j]
			for k := 0; k <= n/N_THREADS; k++ {
				if i >= n {
					break
				}
				procs[i] = proc.NewForkProc(cfg, []string{})
				i++
			}
		}
	} else {
		for i := 0; i < n; i++ {
			procs[i] = proc.NewPythonProc(proc.Python311, []string{THROUGHPUT_BASELINE_SCRIPT})
		}
	}

	return spawnBurstWaitExitProcs(ts, procs)
}

func TestZygoteThroughput(t *testing.T) {
	if N_PROC <= 0 {
		t.Fatalf("nproc must be > 0")
	}
	if N_TRIALS <= 0 {
		t.Fatalf("ntrials must be > 0")
	}

	ts, err := test.NewTstateAll(t)
	if err != nil {
		t.Fatalf("new tstate: %v", err)
	}
	defer ts.Shutdown()

	forkCfg := proc.ForkConfig{
		ZygoteProc: proc.NewPythonProc(proc.Python311, []string{THROUGHPUT_FORK_SCRIPT}),
		KeepAlive:  5 * time.Second,
	}

	baselineResults := benchmarks.NewResults(N_TRIALS, benchmarks.OPS)
	for i := 0; i < N_TRIALS; i++ {
		d := runZygoteThroughputTrial(ts, false, N_PROC, forkCfg)
		baselineResults.Append(d, float64(N_PROC))
		fmt.Printf("baseline trial %d: %v\n", i, d)
	}

	forkResults := benchmarks.NewResults(N_TRIALS, benchmarks.OPS)
	for i := 0; i < N_TRIALS; i++ {
		d := runZygoteThroughputTrial(ts, true, N_PROC, forkCfg)
		forkResults.Append(d, float64(N_PROC))
		fmt.Printf("    fork trial %d: %v\n", i, d)
	}

	bMin, _ := baselineResults.Percentile(0)
	fMin, _ := forkResults.Percentile(0)

	fmt.Printf("\n=== Zygote Throughput ===\n")
	fmt.Printf("nproc=%d ntrials=%d nthreads=%d keepalive=%v\n", N_PROC, N_TRIALS, N_THREADS, ZYGOTE_KEEPALIVE)

	bOpsPerSec := float64(N_PROC) / bMin.Seconds()
	fOpsPerSec := float64(N_PROC) / fMin.Seconds()

	fmt.Printf("baseline: %.2f ops/sec\n", bOpsPerSec)
	fmt.Printf("fork:     %.2f ops/sec\n", fOpsPerSec)
	fmt.Printf("speedup:  %.2fx\n", fOpsPerSec/bOpsPerSec)
}

package benchmarks_test

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"sigmaos/benchmarks"
	"sigmaos/proc"
	"sigmaos/scontainer/python"
	"sigmaos/test"
)

type pythonSPWorkload struct {
	name   string
	script string
}

func getPythonWorkload(name string) (pythonSPWorkload, error) {
	switch name {
	case "massive_import":
		return pythonSPWorkload{name: name, script: "massive_import/main.py"}, nil
	case "numpy_import":
		return pythonSPWorkload{name: name, script: "numpy_import/main.py"}, nil
	default:
		return pythonSPWorkload{}, fmt.Errorf("unknown python workload %q", name)
	}
}

func buildPythonProcSP(w pythonSPWorkload, spType python.TPySitePackagesType) *proc.Proc {
	p := proc.NewPythonProc(proc.Python311, []string{w.script})
	if spType != "" {
		p.Env["SIGMA_PYTHON_SITE_PACKAGES_TYPE"] = string(spType)
	}
	return p
}

func spawnAndWaitPython(ts *test.Tstate, w pythonSPWorkload, spType python.TPySitePackagesType) (time.Duration, error) {
	start := time.Now()
	p := buildPythonProcSP(w, spType)
	if err := ts.Spawn(p); err != nil {
		return 0, fmt.Errorf("spawn: %w", err)
	}
	if err := ts.WaitStart(p.GetPid()); err != nil {
		return 0, fmt.Errorf("waitstart: %w", err)
	}
	st, err := ts.WaitExit(p.GetPid())
	if err != nil {
		return 0, fmt.Errorf("waitexit: %w", err)
	}
	if !st.IsStatusOK() {
		return 0, fmt.Errorf("bad status: %v", st)
	}
	return time.Since(start), nil
}

func printPythonSPStats(label string, rs *benchmarks.Results) {
	mean, _ := rs.Mean()
	std, _ := rs.StdDev()
	p50, _ := rs.Percentile(50)
	p90, _ := rs.Percentile(90)
	p99, _ := rs.Percentile(99)
	p999, _ := rs.Percentile(99.9)
	fmt.Printf("%s mean=%v std=%v p50=%v p90=%v p99=%v p99.9=%v\n", label, mean, std, p50, p90, p99, p999)
}

func TestPythonSitePackagesType(t *testing.T) {
	if PYTHON_NTRIALS <= 0 {
		t.Fatalf("python_ntrials must be > 0")
	}

	w, err := getPythonWorkload(PYTHON_WORKLOAD)
	if err != nil {
		t.Fatal(err)
	}

	ts, err := test.NewTstateAll(t)
	if err != nil {
		t.Fatalf("new tstate: %v", err)
	}
	defer ts.Shutdown()

	if _, err := spawnAndWaitPython(ts, w, ""); err != nil {
		t.Fatalf("warmup: %v", err)
	}

	fmt.Printf("\n=== Python Site Packages Type Benchmark (%s) ===\n", w.name)
	fmt.Printf("trials=%d workload=%s\n", PYTHON_NTRIALS, w.name)

	spTypes := []python.TPySitePackagesType{
		python.OverlaySPType,
		python.SymlinkSPType,
		python.PythonPathSPType,
	}

	results := make(map[python.TPySitePackagesType]*benchmarks.Results)
	for _, spType := range spTypes {
		results[spType] = benchmarks.NewResults(PYTHON_NTRIALS, benchmarks.OPS)
	}

	for _, spType := range spTypes {
		for i := 0; i < PYTHON_NTRIALS; i++ {
			d, err := spawnAndWaitPython(ts, w, spType)
			if err != nil {
				t.Fatalf("trial %d spType=%s: %v", i, spType, err)
			}
			results[spType].Append(d, 1)
		}

		printPythonSPStats(string(spType), results[spType])
	}
}

func parsePyenvWheelLatencies(log string) map[string]time.Duration {
	totals := map[string]time.Duration{}
	opRe := regexp.MustCompile(`(DownloadWheel|InstallWheel).*op:([0-9]+(?:\.[0-9]+)?)(s|ms|us)`)

	scanner := bufio.NewScanner(strings.NewReader(log))
	for scanner.Scan() {
		m := opRe.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}

		val, _ := strconv.ParseFloat(m[2], 64)

		var us float64
		switch m[3] {
		case "s":
			us = val * 1_000_000
		case "ms":
			us = val * 1_000
		case "us":
			us = val
		}

		totals[m[1]] += time.Duration(us) * time.Microsecond
	}

	return totals
}

func TestPythonPyenvDownloadInstallLatency(t *testing.T) {
	benchmarks.EnsureSigmaDebugEnabled(t, "SPAWN_LAT")

	w, err := getPythonWorkload("massive_import")
	if err != nil {
		t.Fatal(err)
	}

	ts, err := test.NewTstateAll(t)
	if err != nil {
		t.Fatalf("new tstate: %v", err)
	}
	defer ts.Shutdown()

	if _, err := spawnAndWaitPython(ts, w, ""); err != nil {
		t.Fatalf("spawnAndWait: %v", err)
	}

	logs, err := runLogsScript()
	if err != nil {
		t.Fatalf("collect logs: %v", err)
	}

	latencies := parsePyenvWheelLatencies(logs)
	for op, latency := range latencies {
		fmt.Printf("%s latency: %v\n", op, latency)
	}

	ratio := float64(latencies["InstallWheel"].Milliseconds()) / float64(latencies["DownloadWheel"].Milliseconds())
	fmt.Printf("Install/Download ratio: %.2f\n", ratio)
}

// func parseStartLatencies(logs string) ([]time.Duration, []time.Duration, error) {
// 	var spawnLatencies []time.Duration
// 	var forkLatencies []time.Duration

// 	re := regexp.MustCompile(`sinceSpawn:(\d+)us`)

// 	lines := strings.Split(logs, "\n")
// 	for _, line := range lines {
// 		if strings.Contains(line, "E2e spawn time since spawn until main") {
// 			m := re.FindStringSubmatch(line)
// 			if m == nil {
// 				return nil, nil, fmt.Errorf("failed to parse spawn latency from line: %s", line)
// 			}

// 			val, err := strconv.Atoi(m[1])
// 			if err != nil {
// 				return nil, nil, fmt.Errorf("failed to parse spawn latency from line: %s", line)
// 			}

// 			spawnLatencies = append(spawnLatencies, time.Duration(val)*time.Microsecond)
// 		} else if strings.Contains(line, "E2e spawn time since fork until main") {
// 			m := re.FindStringSubmatch(line)
// 			if m == nil {
// 				return nil, nil, fmt.Errorf("failed to parse fork latency from line: %s", line)
// 			}

// 			val, err := strconv.Atoi(m[1])
// 			if err != nil {
// 				return nil, nil, fmt.Errorf("failed to parse fork latency from line: %s", line)
// 			}

// 			forkLatencies = append(forkLatencies, time.Duration(val)*time.Microsecond)
// 		}
// 	}

// 	return spawnLatencies, forkLatencies, nil
// }

func TestPythonSitePackagesSpawnLat(t *testing.T) {

}

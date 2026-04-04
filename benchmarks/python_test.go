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
	"sigmaos/test"
)

type pythonImportWorkload struct {
	name   string
	script string
}

func getPythonWorkload(name string) (pythonImportWorkload, error) {
	switch name {
	case "massive_import":
		return pythonImportWorkload{name: name, script: "massive_import/main.py"}, nil
	case "numpy_import":
		return pythonImportWorkload{name: name, script: "numpy_import/main.py"}, nil
	default:
		return pythonImportWorkload{}, fmt.Errorf("unknown python workload %q", name)
	}
}

func buildPythonImportProc(w pythonImportWorkload) *proc.Proc {
	p := proc.NewPythonProc(proc.Python311, []string{w.script})
	return p
}

func spawnAndWaitPython(ts *test.Tstate, w pythonImportWorkload) (time.Duration, error) {
	start := time.Now()
	p := buildPythonImportProc(w)
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

	if _, err := spawnAndWaitPython(ts, w); err != nil {
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

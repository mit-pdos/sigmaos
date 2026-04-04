package scontainer_test

import (
	"flag"
	"fmt"
	"sigmaos/proc"
	"sigmaos/test"
	"sigmaos/util/linux/mem"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var runReverseShell = flag.Bool("reverse-shell", false, "Enable the reverse shell test")

func runBasicPythonTest(ts *test.Tstate, spawn_type string, proc *proc.Proc) {
	start := time.Now()
	err := ts.Spawn(proc)
	assert.Nil(ts.T, err)
	duration := time.Since(start)
	err = ts.WaitStart(proc.GetPid())
	assert.Nil(ts.T, err, "Error waitstart: %v", err)
	duration2 := time.Since(start)
	status, err := ts.WaitExit(proc.GetPid())
	assert.Nil(ts.T, err)
	assert.True(ts.T, status.IsStatusOK(), "Bad exit status: %v", status)
	duration3 := time.Since(start)
	fmt.Printf("%s spawn %v, start %v, exit %v\n", spawn_type, duration, duration2, duration3)
}

func runBasicPythonTestWithoutCheckingExitCode(ts *test.Tstate, spawn_type string, proc *proc.Proc) {
	start := time.Now()
	err := ts.Spawn(proc)
	assert.Nil(ts.T, err)
	duration := time.Since(start)
	err = ts.WaitStart(proc.GetPid())
	assert.Nil(ts.T, err, "Error waitstart: %v", err)
	duration2 := time.Since(start)
	_, _ = ts.WaitExit(proc.GetPid())
	duration3 := time.Since(start)
	fmt.Printf("%s spawn %v, start %v, exit %v\n", spawn_type, duration, duration2, duration3)
}

func TestPythonStartup(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc(proc.Python311, []string{"-c", "exit(1)"})
	runBasicPythonTestWithoutCheckingExitCode(ts, "cold", p)

	p = proc.NewPythonProc(proc.Python311, []string{"-c", "exit(1)"})
	runBasicPythonTestWithoutCheckingExitCode(ts, "warm", p)
}

func TestPythonSplibImport(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	// Launch, connect to sigmaos proxy, signal start & exit
	p := proc.NewPythonProc(proc.Python311, []string{"hello/main.py"})
	runBasicPythonTest(ts, "cold", p)

	p = proc.NewPythonProc(proc.Python311, []string{"hello/main.py"})
	runBasicPythonTest(ts, "warm", p)
}

func TestPythonEnvInfo(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc(proc.Python311, []string{"hello/env_info.py"})
	runBasicPythonTest(ts, "cold", p)
}

func TestPythonBasicImport(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc(proc.Python311, []string{"basic_import/main.py"})
	runBasicPythonTest(ts, "cold", p)
}

func TestPythonNeighborImport(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc(proc.Python311, []string{"neighbor_import/main.py"})
	runBasicPythonTest(ts, "cold", p)
}

func TestPythonNumpyImport(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc(proc.Python311, []string{"numpy_import/main.py"})
	runBasicPythonTest(ts, "cold", p)

	p2 := proc.NewPythonProc(proc.Python311, []string{"numpy_import/main.py"})
	runBasicPythonTest(ts, "warm", p2)
}

func TestImageProcessing(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc(proc.Python311, []string{"imgprocessing/main.py"})
	runBasicPythonTest(ts, "cold", p)
}

func TestPythonReverseShell(t *testing.T) {
	if !*runReverseShell {
		t.Skip("Reverse shell test disabled. Pass --reverse-shell to enable.")
	}

	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	fmt.Printf("To connect to the python reverse shell, run:\n\n  nc -lvnp 4445\n\n")
	p := proc.NewPythonProc(proc.Python311, []string{"_reverse_shell/main.py"})
	runBasicPythonTestWithoutCheckingExitCode(ts, "cold", p)
}

// SigmaOS API Tests

func TestPythonStat(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc(proc.Python311, []string{"stat_test/main.py"})
	runBasicPythonTest(ts, "cold", p)
}

func TestPythonFiles(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	p := proc.NewPythonProc(proc.Python311, []string{"file_test/main.py"})
	runBasicPythonTest(ts, "cold", p)
}

// Fork tests

func TestPythonFork(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	// Create a ForkConfig that specifies how to create a Zygote if needed.
	// The Zygote runs fork/main.py which blocks at splib.fork.fork_point().
	zygote := proc.NewPythonProc(proc.Python311, []string{"fork/main.py"})
	zygote.AppendEnv("PYTHONUNBUFFERED", "1")

	forkConfig := proc.ForkConfig{
		ZygoteProc: zygote,
		KeepAlive:  10 * time.Second,
	}

	// Create the first forked proc
	p1 := proc.NewForkProc(forkConfig, []string{"child1"})

	start := time.Now()
	err := ts.Spawn(p1)
	assert.Nil(t, err)
	duration := time.Since(start)

	err = ts.WaitStart(p1.GetPid())
	assert.Nil(t, err, "Error waitstart: %v", err)
	duration2 := time.Since(start)

	status, err := ts.WaitExit(p1.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK(), "Bad exit status: %v", status)
	duration3 := time.Since(start)
	fmt.Printf("fork1 spawn %v, start %v, exit %v\n", duration, duration2, duration3)

	// Create a second forked proc to reuse the Zygote
	p2 := proc.NewForkProc(forkConfig, []string{"child2"})

	start = time.Now()
	err = ts.Spawn(p2)
	assert.Nil(t, err)
	duration = time.Since(start)

	err = ts.WaitStart(p2.GetPid())
	assert.Nil(t, err, "Error waitstart: %v", err)
	duration2 = time.Since(start)

	status, err = ts.WaitExit(p2.GetPid())
	assert.Nil(t, err)
	assert.True(t, status.IsStatusOK(), "Bad exit status: %v", status)
	duration3 = time.Since(start)
	fmt.Printf("fork2 spawn %v, start %v, exit %v\n", duration, duration2, duration3)
}

func TestPythonForkParallel(t *testing.T) {
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	zygote := proc.NewPythonProc(proc.Python311, []string{"fork/main.py"})
	zygote.AppendEnv("PYTHONUNBUFFERED", "1")

	forkConfig := proc.ForkConfig{
		ZygoteProc: zygote,
		KeepAlive:  10 * time.Second,
	}

	// Warm up the zygote with a single child and print timings.
	pw := proc.NewForkProc(forkConfig, []string{"warm-child"})
	start := time.Now()
	if err := ts.Spawn(pw); err != nil {
		t.Fatalf("warm spawn: %v", err)
	}
	if err := ts.WaitStart(pw.GetPid()); err != nil {
		t.Fatalf("warm waitstart: %v", err)
	}
	warmStartDur := time.Since(start)
	if _, err := ts.WaitExit(pw.GetPid()); err != nil {
		t.Fatalf("warm waitexit: %v", err)
	}
	fmt.Printf("warmup total %v\n", warmStartDur)

	// Now spawn N children in parallel and time the whole round-trip.
	const N = 10
	var wg sync.WaitGroup
	errs := make(chan error, N)
	startAll := time.Now()
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := proc.NewForkProc(forkConfig, []string{fmt.Sprintf("child-%d", i)})
			if err := ts.Spawn(p); err != nil {
				errs <- fmt.Errorf("spawn %d: %w", i, err)
				return
			}
			if err := ts.WaitStart(p.GetPid()); err != nil {
				errs <- fmt.Errorf("waitstart %d: %w", i, err)
				return
			}
			status, err := ts.WaitExit(p.GetPid())
			if err != nil {
				errs <- fmt.Errorf("waitexit %d: %w", i, err)
				return
			}
			if !status.IsStatusOK() {
				errs <- fmt.Errorf("bad exit %d: %v", i, status)
				return
			}
			errs <- nil
		}(i)
	}

	wg.Wait()
	parallelDur := time.Since(startAll)
	close(errs)
	for err := range errs {
		assert.Nil(t, err)
	}
	fmt.Printf("parallel %d total %v\n", N, parallelDur)
}

func TestPythonForkMemory(t *testing.T) {
	// The memory.py script allocates 100 MB of memory.
	ts, _ := test.NewTstateAll(t)
	defer ts.Shutdown()

	const N = 20
	procs := make([]*proc.Proc, N)

	// Without forking
	mema := mem.GetAvailableMem()

	for i := 0; i < N; i++ {
		p := proc.NewPythonProc(proc.Python311, []string{"fork/memory.py"})
		procs[i] = p
		err := ts.Spawn(p)
		assert.Nil(ts.T, err)
		err = ts.WaitStart(p.GetPid())
		assert.Nil(ts.T, err)
	}

	nonForkMem := mema - mem.GetAvailableMem()
	fmt.Printf("Non-forked memory usage for %d procs: %d MB\n", N, nonForkMem)

	for i := 0; i < N; i++ {
		err := ts.WaitEvict(procs[i].GetPid())
		assert.Nil(ts.T, err)
	}

	// With forking
	zygote := proc.NewPythonProc(proc.Python311, []string{"fork/memory.py"})
	forkConfig := proc.ForkConfig{
		ZygoteProc: zygote,
		KeepAlive:  10 * time.Second,
	}

	mema = mem.GetAvailableMem()
	for i := 0; i < N; i++ {
		p := proc.NewForkProc(forkConfig, []string{})
		procs[i] = p
		err := ts.Spawn(p)
		assert.Nil(ts.T, err)
		err = ts.WaitStart(p.GetPid())
		assert.Nil(ts.T, err)
	}

	forkMem := mema - mem.GetAvailableMem()
	fmt.Printf("Forked memory usage for %d procs: %d MB\n", N, forkMem)

	for i := 0; i < N; i++ {
		err := ts.WaitEvict(procs[i].GetPid())
		assert.Nil(ts.T, err)
	}

	// In the ideal scenario, forkMem should be around 1/N of nonForkMem,
	// since most of the memory is shared. However, due to the inaccuracies of
	// measuring memory usage, we use a significantly looser bound.
	assert.True(t, forkMem < nonForkMem/2, "Forked memory usage (%d MB) not less than half of non-forked (%d MB)", forkMem, nonForkMem)
}

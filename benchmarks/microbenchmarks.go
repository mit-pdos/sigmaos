package benchmarks

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/perf"
	"ulambda/proc"
	"ulambda/procclnt"
)

const (
	DEFAULT_N_TRIALS = 1000
	SMALL_FILE_SIZE  = 1 << 10 // 1 KB
	LARGE_FILE_SIZE  = 1 << 20 // 1 MB
	SLEEP_MSECS      = 5
)

const (
	PUT_FILE_DIR = "name/put-file-microbenchmark"
	SET_FILE_DIR = "name/set-file-microbenchmark"
	GET_FILE_DIR = "name/get-file-microbenchmark"
	LOCK_DIR     = "name/lock-microbenchmark"
	SIGNAL_DIR   = "name/signal-microbenchmark"
	FILE_BAG_DIR = "name/filebag-microbenchmark"
)

type Microbenchmarks struct {
	namedAddr []string
	resDir    string
	*fslib.FsLib
	*procclnt.ProcClnt
}

func MakeMicrobenchmarks(fsl *fslib.FsLib, namedAddr []string, resDir string) *Microbenchmarks {
	m := &Microbenchmarks{}
	m.namedAddr = namedAddr
	m.resDir = resDir
	m.FsLib = fsl
	m.ProcClnt = procclnt.MakeProcClnt(m.FsLib)
	return m
}

func (m *Microbenchmarks) RunAll() map[string]*RawResults {
	r := make(map[string]*RawResults)
	r["put_file"] = m.PutFileBenchmark(DEFAULT_N_TRIALS)
	r["set_file_small"] = m.SetFileBenchmark(DEFAULT_N_TRIALS, SMALL_FILE_SIZE)
	for sz := SMALL_FILE_SIZE; sz <= 16*LARGE_FILE_SIZE/20; sz += LARGE_FILE_SIZE / 20 {
		m.SetFileBenchmark(DEFAULT_N_TRIALS*5, sz)
	}
	r["set_file_large"] = m.SetFileBenchmark(DEFAULT_N_TRIALS, LARGE_FILE_SIZE)
	r["get_file_small"] = m.GetFileBenchmark(DEFAULT_N_TRIALS, SMALL_FILE_SIZE)
	r["get_file_large"] = m.GetFileBenchmark(DEFAULT_N_TRIALS, LARGE_FILE_SIZE)
	pidOffset := 0
	r["proc_base_spawn_wait_exit"] = m.ProcBaseSpawnWaitExitBenchmark(DEFAULT_N_TRIALS, pidOffset)
	pidOffset += DEFAULT_N_TRIALS
	r["proc_base_spawn_client"] = m.ProcBaseSpawnClientBenchmark(DEFAULT_N_TRIALS, pidOffset)
	pidOffset += DEFAULT_N_TRIALS
	r["proc_base_exited"] = m.ProcBaseExitedBenchmark(DEFAULT_N_TRIALS, pidOffset)
	pidOffset += DEFAULT_N_TRIALS
	r["proc_base_wait_exit"] = m.ProcBaseWaitExitBenchmark(DEFAULT_N_TRIALS, pidOffset)
	pidOffset += DEFAULT_N_TRIALS
	r["proc_base_linux"] = m.ProcBaseLinuxBenchmark(DEFAULT_N_TRIALS, pidOffset)
	pidOffset += DEFAULT_N_TRIALS
	r["proc_base_pprof"] = m.ProcBasePprofBenchmark(DEFAULT_N_TRIALS*2, pidOffset)
	pidOffset += DEFAULT_N_TRIALS * 2
	return r
}

func (m *Microbenchmarks) setup(dir string) {
	if err := m.Mkdir(dir, 0777); err != nil {
		log.Fatalf("Error Mkdir Microbenchmarks.setup: %v", err)
	}
}

func (m *Microbenchmarks) teardown(dir string) {
	fs, err := m.GetDir(dir)
	if err != nil {
		log.Fatalf("Error ReadDir in Microbenchmarks.teardown: %v", err)
	}

	for _, f := range fs {
		if err := m.Remove(path.Join(dir, f.Name)); err != nil {
			log.Fatalf("Error Remove Microbenchmarks.teardown: %v", err)
		}
	}

	if err := m.Remove(dir); err != nil {
		log.Fatalf("Error Remove in Microbenchmarks.teardown: %v", err)
	}
}

func (m *Microbenchmarks) PutFileBenchmark(nTrials int) *RawResults {
	m.setup(PUT_FILE_DIR)
	defer m.teardown(PUT_FILE_DIR)

	log.Printf("Running PutFileBenchmark...")

	fNames := genFNames(nTrials, PUT_FILE_DIR)

	rs := MakeRawResults(nTrials)

	b := genData(0)
	for i := 0; i < nTrials; i++ {
		nRPC := m.ReadSeqNo()
		start := time.Now()
		if _, err := m.PutFile(fNames[i], 0777, np.OWRITE, b); err != nil {
			log.Fatalf("Error PutFile in Microbenchmarks.PutFileBenchmark: %v", err)
		}
		end := time.Now()
		nRPC = m.ReadSeqNo() - nRPC
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.Data[i].set(throughput, elapsed, nRPC)
	}

	log.Printf("PutFileBenchmark Done")

	return rs
}

func (m *Microbenchmarks) SetFileBenchmark(nTrials int, size int) *RawResults {
	m.setup(SET_FILE_DIR)
	defer m.teardown(SET_FILE_DIR)

	log.Printf("Running SetFileBenchmark (size=%dKB)...", size/(1<<10))

	rs := MakeRawResults(nTrials)

	// Create an empty file
	fpath := path.Join(SET_FILE_DIR, "test-file")
	m.makeFile(fpath, 0)
	b := genData(size)
	for i := 0; i < nTrials; i++ {
		nRPC := m.ReadSeqNo()
		start := time.Now()
		if _, err := m.SetFile(fpath, b, 0); err != nil {
			log.Fatalf("Error SetFile in Microbenchmarks.SetFileBenchmark: %v", err)
		}
		end := time.Now()
		nRPC = m.ReadSeqNo() - nRPC
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.Data[i].set(throughput, elapsed, nRPC)
	}

	log.Printf("SetFileBenchmark Done")

	return rs
}

func (m *Microbenchmarks) GetFileBenchmark(nTrials int, size int) *RawResults {
	m.setup(GET_FILE_DIR)
	defer m.teardown(GET_FILE_DIR)

	log.Printf("Running GetFileBenchmark (size=%dKB)...", size/(1<<10))

	rs := MakeRawResults(nTrials)

	fpath := path.Join(GET_FILE_DIR, "test-file")
	m.makeFile(fpath, size)
	for i := 0; i < nTrials; i++ {
		nRPC := m.ReadSeqNo()
		start := time.Now()
		if _, err := m.GetFile(fpath); err != nil {
			log.Fatalf("Error GetFile in Microbenchmarks.GetFileBenchmark: %v", err)
		}
		end := time.Now()
		nRPC = m.ReadSeqNo() - nRPC
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.Data[i].set(throughput, elapsed, nRPC)
	}

	log.Printf("GetFileBenchmark Done")

	return rs
}

func (m *Microbenchmarks) ProcBaseSpawnWaitExitBenchmark(nTrials int, pidOffset int) *RawResults {
	log.Printf("Running ProcBaseSpawnWaitExitBenchmark...")

	rs := MakeRawResults(nTrials)

	ps := []*proc.Proc{}
	for i := 0; i < nTrials; i++ {
		pid := strconv.Itoa(i + pidOffset)
		p := proc.MakeProcPid(pid, "bin/user/sleeperl", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/out_" + pid})
		ps = append(ps, p)
	}

	for i := 0; i < nTrials; i++ {
		nRPC := m.ReadSeqNo()
		start := time.Now()
		if err := m.Spawn(ps[i]); err != nil {
			log.Fatalf("Error spawning: %v", err)
		}
		if status, err := m.WaitExit(ps[i].Pid); !status.IsStatusOK() || err != nil {
			log.Fatalf("Error WaitExit: %v %v", status, err)
		}
		end := time.Now()
		nRPC = m.ReadSeqNo() - nRPC
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.Data[i].set(throughput, elapsed, nRPC)
	}

	log.Printf("ProcBaseSpawnWaitExitBenchmark Done")

	return rs
}

func (m *Microbenchmarks) ProcBaseLinuxBenchmark(nTrials int, pidOffset int) *RawResults {
	log.Printf("Running ProcBaseLinuxBenchmark...")

	rs := MakeRawResults(nTrials)

	cmds := []*exec.Cmd{}
	for i := 0; i < nTrials; i++ {
		pid := strconv.Itoa(i + pidOffset)
		args := []string{pid, fmt.Sprintf("%dms", SLEEP_MSECS), "name/out_" + pid, "native"}
		env := []string{"NAMED=" + strings.Join(m.namedAddr, ",")}
		cmd := exec.Command("./bin/user/sleeperl", args...)
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmds = append(cmds, cmd)
	}

	for i := 0; i < nTrials; i++ {
		nRPC := m.ReadSeqNo()
		start := time.Now()
		if err := cmds[i].Start(); err != nil {
			log.Fatalf("Error command start: %v", err)
		}
		if err := cmds[i].Wait(); err != nil {
			log.Fatalf("Error command start: %v", err)
		}
		end := time.Now()
		nRPC = m.ReadSeqNo() - nRPC
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.Data[i].set(throughput, elapsed, nRPC)
	}

	log.Printf("ProcBaseLinuxBenchmark Done")

	return rs
}

func (m *Microbenchmarks) ProcBaseSpawnClientBenchmark(nTrials int, pidOffset int) *RawResults {
	log.Printf("Running ProcBaseSpawnClientBenchmark...")

	rs := MakeRawResults(nTrials)

	ps := []*proc.Proc{}
	for i := 0; i < nTrials; i++ {
		pid := strconv.Itoa(i + pidOffset)
		// Note sleep is much shorter, and since we're running "native" the lambda won't actually call Started or Exited for us.
		p := proc.MakeProcPid(pid, "bin/user/sleeperl", []string{fmt.Sprintf("%dus", SLEEP_MSECS), "name/out_" + pid, "native"})
		ps = append(ps, p)
	}

	for i := 0; i < nTrials; i++ {
		nRPC := m.ReadSeqNo()
		start := time.Now()
		if err := m.Spawn(ps[i]); err != nil {
			log.Fatalf("Error spawning: %v", err)
		}
		end := time.Now()
		nRPC = m.ReadSeqNo() - nRPC
		m.Exited(ps[i].Pid, proc.MakeStatus(proc.StatusOK))
		if status, err := m.WaitExit(ps[i].Pid); !status.IsStatusOK() || err != nil {
			log.Fatalf("Error WaitExit: %v %v", status, err)
		}
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.Data[i].set(throughput, elapsed, nRPC)
	}

	log.Printf("ProcBaseSpawnClientBenchmark Done")

	return rs
}

func (m *Microbenchmarks) ProcBaseExitedBenchmark(nTrials int, pidOffset int) *RawResults {
	log.Printf("Running ProcBaseExitedBenchmark...")

	rs := MakeRawResults(nTrials)

	ps := []*proc.Proc{}
	for i := 0; i < nTrials; i++ {
		pid := strconv.Itoa(i + pidOffset)
		// Note sleep is much shorter, and since we're running "native" the lambda won't actually call Started or Exited for us.
		p := proc.MakeProcPid(pid, "bin/user/sleeperl", []string{fmt.Sprintf("%dus", SLEEP_MSECS), "name/out_" + pid, "native"})
		ps = append(ps, p)
	}

	for i := 0; i < nTrials; i++ {
		if err := m.Spawn(ps[i]); err != nil {
			log.Fatalf("Error spawning: %v", err)
		}
		nRPC := m.ReadSeqNo()
		start := time.Now()
		m.Exited(ps[i].Pid, proc.MakeStatus(proc.StatusOK))
		end := time.Now()
		nRPC = m.ReadSeqNo() - nRPC
		if status, err := m.WaitExit(ps[i].Pid); !status.IsStatusOK() || err != nil {
			log.Fatalf("Error WaitExit: %v %v", status, err)
		}
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.Data[i].set(throughput, elapsed, nRPC)
	}

	log.Printf("ProcBaseExitedBenchmark Done")

	return rs
}

func (m *Microbenchmarks) ProcBaseWaitExitBenchmark(nTrials int, pidOffset int) *RawResults {
	log.Printf("Running ProcBaseWaitExitBenchmark...")

	rs := MakeRawResults(nTrials)

	ps := []*proc.Proc{}
	for i := 0; i < nTrials; i++ {
		pid := strconv.Itoa(i + pidOffset)
		// Note sleep is much shorter, and since we're running "native" the lambda won't actually call Started or Exited for us.
		p := proc.MakeProcPid(pid, "bin/user/sleeperl", []string{fmt.Sprintf("%dus", SLEEP_MSECS), "name/out_" + pid, "native"})
		ps = append(ps, p)
	}

	for i := 0; i < nTrials; i++ {
		if err := m.Spawn(ps[i]); err != nil {
			log.Fatalf("Error spawning: %v", err)
		}
		m.Exited(ps[i].Pid, proc.MakeStatus(proc.StatusOK))
		nRPC := m.ReadSeqNo()
		start := time.Now()
		if status, err := m.WaitExit(ps[i].Pid); !status.IsStatusOK() || err != nil {
			log.Fatalf("Error WaitExit: %v %v", status, err)
		}
		end := time.Now()
		nRPC = m.ReadSeqNo() - nRPC
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.Data[i].set(throughput, elapsed, nRPC)
	}

	log.Printf("ProcBaseWaitExitBenchmark Done")

	return rs
}

func (m *Microbenchmarks) ProcBasePprofBenchmark(nTrials int, pidOffset int) *RawResults {
	log.Printf("Running ProcBasePprofBenchmark...")

	// Start pprof to break down costs
	runtime.SetCPUProfileRate(250)
	pprofPath := path.Join(m.resDir, "pprof", "procbase.txt")
	p := perf.MakePerf()
	p.SetupPprof(pprofPath)
	defer p.Teardown()

	rs := MakeRawResults(nTrials)

	ps := []*proc.Proc{}
	for i := 0; i < nTrials; i++ {
		pid := strconv.Itoa(i + pidOffset)
		// Note sleep is much shorter, and since we're running "native" the lambda won't actually call Started or Exited for us.
		p := proc.MakeProcPid(pid, "bin/user/sleeperl", []string{fmt.Sprintf("%dus", SLEEP_MSECS), "name/out_" + pid, "native"})
		ps = append(ps, p)
	}

	for i := 0; i < nTrials; i++ {
		nRPC := m.ReadSeqNo()
		start := time.Now()
		if err := m.Spawn(ps[i]); err != nil {
			log.Fatalf("Error spawning: %v", err)
		}
		m.Exited(ps[i].Pid, proc.MakeStatus(proc.StatusOK))
		if status, err := m.WaitExit(ps[i].Pid); !status.IsStatusOK() || err != nil {
			log.Fatalf("Error WaitExit: %v %v", status, err)
		}
		end := time.Now()
		nRPC = m.ReadSeqNo() - nRPC
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.Data[i].set(throughput, elapsed, nRPC)
	}

	log.Printf("ProcBasePprofBenchmark Done")

	return rs
}

func (m *Microbenchmarks) makeFile(fpath string, size int) {
	b := genData(size)
	if _, err := m.PutFile(fpath, 0777, np.OWRITE, b); err != nil {
		log.Fatalf("Error MakeFile Microbenchmarks.makeFile: %v", err)
	}
}

func genFNames(nTrials int, dir string) []string {
	fNames := make([]string, nTrials)
	for i := 0; i < nTrials; i++ {
		fNames[i] = path.Join(dir, strconv.Itoa(i))
	}
	return fNames
}

func genData(size int) []byte {
	b := make([]byte, size)
	for i := 0; i < size; i++ {
		b[i] = 'a'
	}
	return b
}

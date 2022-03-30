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

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/perf"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/semclnt"
)

const (
	DEFAULT_N_TRIALS = 1000
	SMALL_FILE_SIZE  = 1 << 10     // 1 KB
	LARGE_FILE_SIZE  = 1 << 20 / 2 // 1 MB
	SLEEP_MSECS      = 5
)

const (
	PUT_FILE_DIR = "name/put-file-microbenchmark"
	SET_FILE_DIR = "name/set-file-microbenchmark"
	GET_FILE_DIR = "name/get-file-microbenchmark"
	SEM_DIR      = "name/sem-microbenchmark"
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
	m.ProcClnt = procclnt.MakeProcClntInit(m.FsLib, "microbenchmarks", namedAddr)
	return m
}

func (m *Microbenchmarks) RunAll() map[string]*RawResults {
	r := make(map[string]*RawResults)
	//	r["put_file"] = m.PutFileBenchmark(DEFAULT_N_TRIALS)
	//	r["set_file_small"] = m.SetFileBenchmark(DEFAULT_N_TRIALS, SMALL_FILE_SIZE)
	//	for sz := SMALL_FILE_SIZE; sz < 16*LARGE_FILE_SIZE/20; sz += LARGE_FILE_SIZE / 10 {
	//		m.SetFileBenchmark(DEFAULT_N_TRIALS*5, sz)
	//	}
	//	r["set_file_large"] = m.SetFileBenchmark(DEFAULT_N_TRIALS, LARGE_FILE_SIZE)
	//	r["get_file_small"] = m.GetFileBenchmark(DEFAULT_N_TRIALS, SMALL_FILE_SIZE)
	//	r["get_file_large"] = m.GetFileBenchmark(DEFAULT_N_TRIALS, LARGE_FILE_SIZE)
	//	r["sem_init"] = m.SemInitBenchmark(DEFAULT_N_TRIALS)
	//	r["sem_up"] = m.SemUpBenchmark(DEFAULT_N_TRIALS)
	//	r["sem_down"] = m.SemDownBenchmark(DEFAULT_N_TRIALS)
	pidOffset := 0
	r["proc_base_spawn_wait_exit"] = m.ProcSpawnWaitExitBenchmark(DEFAULT_N_TRIALS, pidOffset)
	pidOffset += DEFAULT_N_TRIALS
	r["proc_base_spawn_client"] = m.ProcSpawnClientBenchmark(DEFAULT_N_TRIALS, pidOffset)
	pidOffset += DEFAULT_N_TRIALS
	//	r["proc_base_exited"] = m.ProcExitedBenchmark(DEFAULT_N_TRIALS, pidOffset)
	//	pidOffset += DEFAULT_N_TRIALS
	//	r["proc_base_wait_exit"] = m.ProcWaitExitBenchmark(DEFAULT_N_TRIALS, pidOffset)
	//	pidOffset += DEFAULT_N_TRIALS
	//	r["proc_base_linux"] = m.ProcLinuxBenchmark(DEFAULT_N_TRIALS, pidOffset)
	//	pidOffset += DEFAULT_N_TRIALS
	//	r["proc_base_pprof"] = m.ProcPprofBenchmark(DEFAULT_N_TRIALS*2, pidOffset)
	//	pidOffset += DEFAULT_N_TRIALS * 2
	return r
}

func (m *Microbenchmarks) setup(dir string) {
	if err := m.MkDir(dir, 0777); err != nil {
		db.DFatalf("Error Mkdir Microbenchmarks.setup: %v", err)
	}
}

func (m *Microbenchmarks) teardown(dir string) {
	fs, err := m.GetDir(dir)
	if err != nil {
		db.DFatalf("Error ReadDir in Microbenchmarks.teardown: %v", err)
	}

	for _, f := range fs {
		if err := m.Remove(path.Join(dir, f.Name)); err != nil {
			db.DFatalf("Error Remove Microbenchmarks.teardown: %v", err)
		}
	}

	if err := m.Remove(dir); err != nil {
		db.DFatalf("Error Remove in Microbenchmarks.teardown: %v", err)
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
			db.DFatalf("Error PutFile in Microbenchmarks.PutFileBenchmark: %v", err)
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
		if _, err := m.SetFile(fpath, b, np.OWRITE, 0); err != nil {
			db.DFatalf("Error SetFile in Microbenchmarks.SetFileBenchmark: %v", err)
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
			db.DFatalf("Error GetFile in Microbenchmarks.GetFileBenchmark: %v", err)
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

func (m *Microbenchmarks) SemInitBenchmark(nTrials int) *RawResults {
	m.setup(SEM_DIR)
	defer m.teardown(SEM_DIR)

	log.Printf("Running SemInitBenchmark...")

	sems := genSems(m.FsLib, nTrials, SEM_DIR)
	rs := MakeRawResults(nTrials)

	for i := 0; i < nTrials; i++ {
		nRPC := m.ReadSeqNo()
		start := time.Now()
		if err := sems[i].Init(0); err != nil {
			db.DFatalf("Error PutFile in Microbenchmarks.SemInitBenchmark: %v", err)
		}
		end := time.Now()
		nRPC = m.ReadSeqNo() - nRPC
		if err := sems[i].Up(); err != nil {
			db.DFatalf("Error PutFile in Microbenchmarks.SemInitBenchmark: %v", err)
		}
		if err := sems[i].Down(); err != nil {
			db.DFatalf("Error PutFile in Microbenchmarks.SemInitBenchmark: %v", err)
		}
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.Data[i].set(throughput, elapsed, nRPC)
	}

	log.Printf("SemInitBenchmark Done")

	return rs
}

func (m *Microbenchmarks) SemUpBenchmark(nTrials int) *RawResults {
	m.setup(SEM_DIR)
	defer m.teardown(SEM_DIR)

	log.Printf("Running SemUpBenchmark...")

	sems := genSems(m.FsLib, nTrials, SEM_DIR)
	rs := MakeRawResults(nTrials)

	for i := 0; i < nTrials; i++ {
		if err := sems[i].Init(0); err != nil {
			db.DFatalf("Error PutFile in Microbenchmarks.SemUpBenchmark: %v", err)
		}
		nRPC := m.ReadSeqNo()
		start := time.Now()
		if err := sems[i].Up(); err != nil {
			db.DFatalf("Error PutFile in Microbenchmarks.SemUpBenchmark: %v", err)
		}
		end := time.Now()
		nRPC = m.ReadSeqNo() - nRPC
		if err := sems[i].Down(); err != nil {
			db.DFatalf("Error PutFile in Microbenchmarks.SemUpBenchmark: %v", err)
		}
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.Data[i].set(throughput, elapsed, nRPC)
	}

	log.Printf("SemUpBenchmark Done")

	return rs
}

func (m *Microbenchmarks) SemDownBenchmark(nTrials int) *RawResults {
	m.setup(SEM_DIR)
	defer m.teardown(SEM_DIR)

	log.Printf("Running SemDownBenchmark...")

	sems := genSems(m.FsLib, nTrials, SEM_DIR)
	rs := MakeRawResults(nTrials)

	for i := 0; i < nTrials; i++ {
		if err := sems[i].Init(0); err != nil {
			db.DFatalf("Error PutFile in Microbenchmarks.SemDownBenchmark: %v", err)
		}
		if err := sems[i].Up(); err != nil {
			db.DFatalf("Error PutFile in Microbenchmarks.SemDownBenchmark: %v", err)
		}
		nRPC := m.ReadSeqNo()
		start := time.Now()
		if err := sems[i].Down(); err != nil {
			db.DFatalf("Error PutFile in Microbenchmarks.SemDownBenchmark: %v", err)
		}
		end := time.Now()
		nRPC = m.ReadSeqNo() - nRPC
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.Data[i].set(throughput, elapsed, nRPC)
	}

	log.Printf("SemDownBenchmark Done")

	return rs
}

func (m *Microbenchmarks) ProcSpawnWaitExitBenchmark(nTrials int, pidOffset int) *RawResults {
	log.Printf("Running ProcSpawnWaitExitBenchmark...")

	rs := MakeRawResults(nTrials)

	ps := []*proc.Proc{}
	for i := 0; i < nTrials; i++ {
		pid := proc.Tpid(strconv.Itoa(i + pidOffset))
		p := proc.MakeProcPid(pid, "bin/user/sleeper", []string{fmt.Sprintf("%dms", SLEEP_MSECS), "name/out_" + pid.String()})
		ps = append(ps, p)
	}

	for i := 0; i < nTrials; i++ {
		nRPC := m.ReadSeqNo()
		start := time.Now()
		if err := m.Spawn(ps[i]); err != nil {
			db.DFatalf("Error spawning: %v", err)
		}
		if status, err := m.WaitExit(ps[i].Pid); !status.IsStatusOK() || err != nil {
			db.DFatalf("Error WaitExit: %v %v", status, err)
		}
		end := time.Now()
		nRPC = m.ReadSeqNo() - nRPC
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.Data[i].set(throughput, elapsed, nRPC)
	}

	log.Printf("ProcSpawnWaitExitBenchmark Done")

	return rs
}

func (m *Microbenchmarks) ProcLinuxBenchmark(nTrials int, pidOffset int) *RawResults {
	log.Printf("Running ProcLinuxBenchmark...")

	rs := MakeRawResults(nTrials)

	cmds := []*exec.Cmd{}
	for i := 0; i < nTrials; i++ {
		pid := strconv.Itoa(i + pidOffset)
		args := []string{pid, fmt.Sprintf("%dms", SLEEP_MSECS), "name/out_" + pid, "native"}
		env := []string{"NAMED=" + strings.Join(m.namedAddr, ",")}
		cmd := exec.Command("./bin/user/sleeper", args...)
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmds = append(cmds, cmd)
	}

	for i := 0; i < nTrials; i++ {
		nRPC := m.ReadSeqNo()
		start := time.Now()
		if err := cmds[i].Start(); err != nil {
			db.DFatalf("Error command start: %v", err)
		}
		if err := cmds[i].Wait(); err != nil {
			db.DFatalf("Error command start: %v", err)
		}
		end := time.Now()
		nRPC = m.ReadSeqNo() - nRPC
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.Data[i].set(throughput, elapsed, nRPC)
	}

	log.Printf("ProcLinuxBenchmark Done")

	return rs
}

func (m *Microbenchmarks) ProcSpawnClientBenchmark(nTrials int, pidOffset int) *RawResults {
	log.Printf("Running ProcSpawnClientBenchmark...")

	rs := MakeRawResults(nTrials)

	ps := []*proc.Proc{}
	for i := 0; i < nTrials; i++ {
		pid := proc.Tpid(strconv.Itoa(i + pidOffset))
		// Note sleep is much shorter, and since we're running "native" the lambda won't actually call Started or Exited for us.
		p := proc.MakeProcPid(pid, "bin/user/sleeper", []string{fmt.Sprintf("%dus", SLEEP_MSECS), "name/out_" + pid.String()})
		ps = append(ps, p)
	}

	for i := 0; i < nTrials; i++ {
		nRPC := m.ReadSeqNo()
		start := time.Now()
		if err := m.Spawn(ps[i]); err != nil {
			db.DFatalf("Error spawning: %v", err)
		}
		end := time.Now()
		nRPC = m.ReadSeqNo() - nRPC
		//		m.Exited(ps[i].Pid, proc.MakeStatus(proc.StatusOK))
		if status, err := m.WaitExit(ps[i].Pid); !status.IsStatusOK() || err != nil {
			db.DFatalf("Error WaitExit: %v %v", status, err)
		}
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.Data[i].set(throughput, elapsed, nRPC)
	}

	log.Printf("ProcSpawnClientBenchmark Done")

	return rs
}

func (m *Microbenchmarks) ProcExitedBenchmark(nTrials int, pidOffset int) *RawResults {
	log.Printf("Running ProcExitedBenchmark...")

	rs := MakeRawResults(nTrials)

	ps := []*proc.Proc{}
	for i := 0; i < nTrials; i++ {
		pid := proc.Tpid(strconv.Itoa(i + pidOffset))
		// Note sleep is much shorter, and since we're running "native" the lambda won't actually call Started or Exited for us.
		p := proc.MakeProcPid(pid, "bin/user/sleeper", []string{fmt.Sprintf("%dus", SLEEP_MSECS), "name/out_" + pid.String(), "native"})
		ps = append(ps, p)
	}

	for i := 0; i < nTrials; i++ {
		if err := m.Spawn(ps[i]); err != nil {
			db.DFatalf("Error spawning: %v", err)
		}
		nRPC := m.ReadSeqNo()
		start := time.Now()
		//		m.Exited(ps[i].Pid, proc.MakeStatus(proc.StatusOK))
		end := time.Now()
		nRPC = m.ReadSeqNo() - nRPC
		if status, err := m.WaitExit(ps[i].Pid); !status.IsStatusOK() || err != nil {
			db.DFatalf("Error WaitExit: %v %v", status, err)
		}
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.Data[i].set(throughput, elapsed, nRPC)
	}

	log.Printf("ProcExitedBenchmark Done")

	return rs
}

func (m *Microbenchmarks) ProcWaitExitBenchmark(nTrials int, pidOffset int) *RawResults {
	log.Printf("Running ProcWaitExitBenchmark...")

	rs := MakeRawResults(nTrials)

	ps := []*proc.Proc{}
	for i := 0; i < nTrials; i++ {
		pid := proc.Tpid(strconv.Itoa(i + pidOffset))
		// Note sleep is much shorter, and since we're running "native" the lambda won't actually call Started or Exited for us.
		p := proc.MakeProcPid(pid, "bin/user/sleeper", []string{fmt.Sprintf("%dus", SLEEP_MSECS), "name/out_" + pid.String(), "native"})
		ps = append(ps, p)
	}

	for i := 0; i < nTrials; i++ {
		if err := m.Spawn(ps[i]); err != nil {
			db.DFatalf("Error spawning: %v", err)
		}
		//		m.Exited(ps[i].Pid, proc.MakeStatus(proc.StatusOK))
		nRPC := m.ReadSeqNo()
		start := time.Now()
		if status, err := m.WaitExit(ps[i].Pid); !status.IsStatusOK() || err != nil {
			db.DFatalf("Error WaitExit: %v %v", status, err)
		}
		end := time.Now()
		nRPC = m.ReadSeqNo() - nRPC
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.Data[i].set(throughput, elapsed, nRPC)
	}

	log.Printf("ProcWaitExitBenchmark Done")

	return rs
}

func (m *Microbenchmarks) ProcPprofBenchmark(nTrials int, pidOffset int) *RawResults {
	log.Printf("Running ProcPprofBenchmark...")

	// Start pprof to break down costs
	runtime.SetCPUProfileRate(250)
	pprofPath := path.Join(m.resDir, "pprof", "procbase.txt")
	p := perf.MakePerf()
	p.SetupPprof(pprofPath)
	defer p.Teardown()

	rs := MakeRawResults(nTrials)

	ps := []*proc.Proc{}
	for i := 0; i < nTrials; i++ {
		pid := proc.Tpid(strconv.Itoa(i + pidOffset))
		// Note sleep is much shorter, and since we're running "native" the lambda won't actually call Started or Exited for us.
		p := proc.MakeProcPid(pid, "bin/user/sleeper", []string{fmt.Sprintf("%dus", SLEEP_MSECS), "name/out_" + pid.String(), "native"})
		ps = append(ps, p)
	}

	for i := 0; i < nTrials; i++ {
		nRPC := m.ReadSeqNo()
		start := time.Now()
		if err := m.Spawn(ps[i]); err != nil {
			db.DFatalf("Error spawning: %v", err)
		}
		//		m.Exited(ps[i].Pid, proc.MakeStatus(proc.StatusOK))
		if status, err := m.WaitExit(ps[i].Pid); !status.IsStatusOK() || err != nil {
			db.DFatalf("Error WaitExit: %v %v", status, err)
		}
		end := time.Now()
		nRPC = m.ReadSeqNo() - nRPC
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.Data[i].set(throughput, elapsed, nRPC)
	}

	log.Printf("ProcPprofBenchmark Done")

	return rs
}

func (m *Microbenchmarks) makeFile(fpath string, size int) {
	b := genData(size)
	if _, err := m.PutFile(fpath, 0777, np.OWRITE, b); err != nil {
		db.DFatalf("Error MakeFile Microbenchmarks.makeFile: %v", err)
	}
}

func genSems(fsl *fslib.FsLib, nTrials int, dir string) []*semclnt.SemClnt {
	fnames := genFNames(nTrials, dir)
	sems := []*semclnt.SemClnt{}
	for _, n := range fnames {
		sems = append(sems, semclnt.MakeSemClnt(fsl, n))
	}
	return sems
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

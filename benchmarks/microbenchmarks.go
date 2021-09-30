package benchmarks

import (
	"log"
	"path"
	"strconv"
	"time"

	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/sync"
)

const (
	DEFAULT_N_TRIALS = 1000
	SMALL_FILE_SIZE  = 1 << 10 // 1 KB
	LARGE_FILE_SIZE  = 1 << 20 // 1 MB
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
	*fslib.FsLib
}

func MakeMicrobenchmarks(fsl *fslib.FsLib) *Microbenchmarks {
	m := &Microbenchmarks{}
	m.FsLib = fsl
	return m
}

func (m *Microbenchmarks) RunAll() map[string]*RawResults {
	r := make(map[string]*RawResults)
	r["put_file"] = m.PutFileBenchmark(DEFAULT_N_TRIALS)
	r["set_file_small"] = m.SetFileBenchmark(DEFAULT_N_TRIALS, SMALL_FILE_SIZE)
	r["set_file_large"] = m.SetFileBenchmark(DEFAULT_N_TRIALS, LARGE_FILE_SIZE)
	r["get_file_small"] = m.GetFileBenchmark(DEFAULT_N_TRIALS, SMALL_FILE_SIZE)
	r["get_file_large"] = m.GetFileBenchmark(DEFAULT_N_TRIALS, LARGE_FILE_SIZE)
	r["lock_lock"] = m.LockLockBenchmark(DEFAULT_N_TRIALS)
	r["lock_unlock"] = m.LockUnlockBenchmark(DEFAULT_N_TRIALS)
	//	r["signal_signal"] = m.SignalSignalBenchmark(DEFAULT_N_TRIALS)
	//	r["signal_wait"] = m.SignalWaitBenchmark(DEFAULT_N_TRIALS)
	r["file_bag_put"] = m.FileBagPutBenchmark(DEFAULT_N_TRIALS, SMALL_FILE_SIZE)
	r["file_bag_get"] = m.FileBagGetBenchmark(DEFAULT_N_TRIALS, SMALL_FILE_SIZE)
	return r
}

func (m *Microbenchmarks) setup(dir string) {
	if err := m.Mkdir(dir, 0777); err != nil {
		log.Fatalf("Error Mkdir Microbenchmarks.setup: %v", err)
	}
}

func (m *Microbenchmarks) teardown(dir string) {
	fs, err := m.ReadDir(dir)
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
		start := time.Now()
		if _, err := m.PutFile(fNames[i], b, 0777, np.OWRITE); err != nil {
			log.Fatalf("Error PutFile in Microbenchmarks.PutFileBenchmark: %v", err)
		}
		end := time.Now()
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.data[i].set(throughput, elapsed)
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
		start := time.Now()
		if _, err := m.SetFile(fpath, b, np.NoV); err != nil {
			log.Fatalf("Error SetFile in Microbenchmarks.SetFileBenchmark: %v", err)
		}
		end := time.Now()
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.data[i].set(throughput, elapsed)
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
		start := time.Now()
		if _, _, err := m.GetFile(fpath); err != nil {
			log.Fatalf("Error GetFile in Microbenchmarks.GetFileBenchmark: %v", err)
		}
		end := time.Now()
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.data[i].set(throughput, elapsed)
	}

	log.Printf("GetFileBenchmark Done")

	return rs
}

func (m *Microbenchmarks) LockLockBenchmark(nTrials int) *RawResults {
	m.setup(LOCK_DIR)
	defer m.teardown(LOCK_DIR)

	log.Printf("Running LockLockBenchmark...")

	lName := "test-lock"

	l := sync.MakeLock(m.FsLib, LOCK_DIR, lName, true)

	rs := MakeRawResults(nTrials)

	for i := 0; i < nTrials; i++ {
		start := time.Now()
		l.Lock()
		end := time.Now()
		l.Unlock()
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.data[i].set(throughput, elapsed)
	}

	log.Printf("LockLockBenchmark Done")

	return rs
}

func (m *Microbenchmarks) LockUnlockBenchmark(nTrials int) *RawResults {
	m.setup(LOCK_DIR)
	defer m.teardown(LOCK_DIR)

	log.Printf("Running LockUnlockBenchmark...")

	lName := "test-lock"

	l := sync.MakeLock(m.FsLib, LOCK_DIR, lName, true)

	rs := MakeRawResults(nTrials)

	for i := 0; i < nTrials; i++ {
		l.Lock()
		start := time.Now()
		l.Unlock()
		end := time.Now()
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.data[i].set(throughput, elapsed)
	}

	log.Printf("LockUnlockBenchmark Done")

	return rs
}

//func (m *Microbenchmarks) SignalSignalBenchmark(nTrials int) *RawResults {
//	m.setup(SIGNAL_DIR)
//	defer m.teardown(SIGNAL_DIR)
//
//	log.Printf("Running SignalSignalBenchmark...")
//
//	lName := "test-lock"
//
//	l := sync.MakeLock(m.FsLib, LOCK_DIR, lName, true)
//
//	rs := MakeRawResults(nTrials)
//
//	for i := 0; i < nTrials; i++ {
//		l.Lock()
//		start := time.Now()
//		l.Unlock()
//		end := time.Now()
//		elapsed := float64(end.Sub(start).Microseconds())
//		throughput := float64(1.0) / elapsed
//		rs.data[i].set(throughput, elapsed)
//	}
//
//	log.Printf("SignalSignalBenchmark Done")
//
//	return rs
//}

func (m *Microbenchmarks) FileBagPutBenchmark(nTrials int, size int) *RawResults {
	m.setup(FILE_BAG_DIR)
	defer m.teardown(FILE_BAG_DIR)

	log.Printf("Running FileBagPutBenchmark (size=%dKB)...", size/(1<<10))

	rs := MakeRawResults(nTrials)

	path := path.Join(FILE_BAG_DIR, "filebag")
	priority := "1"
	name := "abcd"
	bag := sync.MakeFilePriorityBag(m.FsLib, path)
	b := genData(size)
	for i := 0; i < nTrials; i++ {
		start := time.Now()
		if err := bag.Put(priority, name, b); err != nil {
			log.Fatalf("Error Put in Microbenchmarks.FileBagPutBenchmark: %v", err)
		}
		end := time.Now()
		if _, _, _, err := bag.Get(); err != nil {
			log.Fatalf("Error Get in Microbenchmarks.FileBagPutBenchmark: %v", err)
		}
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.data[i].set(throughput, elapsed)
	}

	log.Printf("FileBagPutBenchmark Done")

	return rs
}

func (m *Microbenchmarks) FileBagGetBenchmark(nTrials int, size int) *RawResults {
	m.setup(FILE_BAG_DIR)
	defer m.teardown(FILE_BAG_DIR)

	log.Printf("Running FileBagGetBenchmark (size=%dKB)...", size/(1<<10))

	rs := MakeRawResults(nTrials)

	path := path.Join(FILE_BAG_DIR, "filebag")
	priority := "1"
	name := "abcd"
	bag := sync.MakeFilePriorityBag(m.FsLib, path)
	b := genData(size)
	for i := 0; i < nTrials; i++ {
		if err := bag.Put(priority, name, b); err != nil {
			log.Fatalf("Error Put in Microbenchmarks.FileBagGetBenchmark: %v", err)
		}
		start := time.Now()
		if _, _, _, err := bag.Get(); err != nil {
			log.Fatalf("Error Get in Microbenchmarks.FileBagGetBenchmark: %v", err)
		}
		end := time.Now()
		elapsed := float64(end.Sub(start).Microseconds())
		throughput := float64(1.0) / elapsed
		rs.data[i].set(throughput, elapsed)
	}

	log.Printf("FileBagGetBenchmark Done")

	return rs
}

func (m *Microbenchmarks) makeFile(fpath string, size int) {
	b := genData(size)
	if err := m.MakeFile(fpath, 0777, np.OWRITE, b); err != nil {
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

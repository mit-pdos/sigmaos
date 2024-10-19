package mr

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	// "runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/rand"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	MAXCAP      = 32
	MINCAP      = 4
	CONCURRENCY = 5
)

type Mapper struct {
	*sigmaclnt.SigmaClnt
	mapf        MapT
	combinef    ReduceT
	sbc         *ScanByteCounter
	jobRoot     string
	job         string
	nreducetask int
	linesz      int
	input       string
	intOutput   string
	bin         string
	wrts        []*fslib.FileWriter
	pwrts       []*perf.PerfWriter
	rand        string
	perf        *perf.Perf
	asyncrw     bool
	combined    *kvmap
	combinewc   map[string]int
	bufs        [][]byte
	lines       [][]byte
	init        bool
	ch          chan error
}

func NewMapper(sc *sigmaclnt.SigmaClnt, mapf MapT, combinef ReduceT, jobRoot, job string, p *perf.Perf, nr, lsz int, input, intOutput string) (*Mapper, error) {
	m := &Mapper{
		SigmaClnt:   sc,
		mapf:        mapf,
		combinef:    combinef,
		jobRoot:     jobRoot,
		job:         job,
		nreducetask: nr,
		linesz:      lsz,
		rand:        rand.String(16),
		input:       input,
		intOutput:   intOutput,
		bin:         filepath.Base(input),
		wrts:        make([]*fslib.FileWriter, nr),
		pwrts:       make([]*perf.PerfWriter, nr),
		perf:        p,
		sbc:         NewScanByteCounter(p),
		combined:    newKvmap(MINCAP, MAXCAP),
		combinewc:   make(map[string]int),
		bufs:        make([][]byte, CONCURRENCY),
		lines:       make([][]byte, CONCURRENCY),
		ch:          make(chan error),
	}
	for i := 0; i < CONCURRENCY; i++ {
		m.bufs[i] = make([]byte, 0, lsz)
		m.lines[i] = make([]byte, 0, lsz)
	}
	m.MountS3PathClnt()
	go func() {
		m.ch <- m.initOutput()
	}()
	return m, nil
}

func newMapper(mapf MapT, reducef ReduceT, args []string, p *perf.Perf) (*Mapper, error) {

	if len(args) != 6 {
		return nil, fmt.Errorf("NewMapper: too few arguments %v", args)
	}
	nr, err := strconv.Atoi(args[2])
	if err != nil {
		return nil, fmt.Errorf("NewMapper: nreducetask %v isn't int", args[2])
	}
	lsz, err := strconv.Atoi(args[5])
	if err != nil {
		return nil, fmt.Errorf("NewMapper: linesz %v isn't int", args[2])
	}
	start := time.Now()
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}

	db.DPrintf(db.TEST, "NewSigmaClnt done at time: %v", time.Since(start))
	m, err := NewMapper(sc, mapf, reducef, args[0], args[1], p, nr, lsz, args[3], args[4])
	if err != nil {
		return nil, fmt.Errorf("NewMapper failed %v", err)
	}

	db.DPrintf(db.TEST, "NewMapper done at time: %v", time.Since(start))
	if err := m.Started(); err != nil {
		return nil, fmt.Errorf("NewMapper couldn't start %v", args)
	}
	db.DPrintf(db.TEST, "Started at time: %v", time.Since(start))
	crash.Crasher(m.FsLib)
	return m, nil
}

func (m *Mapper) CloseWrt() (sp.Tlength, error) {
	nout, err := m.closewrts()
	if err != nil {
		return 0, err
	}
	return nout, nil
}

func (m *Mapper) initWrt(r int, name string) error {
	pn, ok := sp.S3ClientPath(name)
	if ok {
		name = pn
	}
	db.DPrintf(db.MR, "InitWrt %v", name)
	if wrt, err := m.CreateBufWriter(name, 0777); err != nil {
		return err
	} else {
		m.wrts[r] = wrt
		m.pwrts[r] = perf.NewPerfWriter(wrt, m.perf)
	}
	return nil
}

func (m *Mapper) initOutput() error {
	start := time.Now()
	defer func(start time.Time) {
		db.DPrintf(db.TEST, "initOutput time: %v", time.Since(start))
	}(start)

	outDirPath := MapIntermediateDir(m.job, m.intOutput)

	// Create the output files
	for r := 0; r < m.nreducetask; r++ {
		// create temp output shard for reducer r
		oname := mshardfile(outDirPath, r) + m.rand
		if err := m.initWrt(r, oname); err != nil {
			m.closewrts()
			return err
		}
	}
	return nil
}

func (m *Mapper) closewrts() (sp.Tlength, error) {
	n := sp.Tlength(0)
	for r := 0; r < m.nreducetask; r++ {
		if m.wrts[r] != nil {
			if err := m.wrts[r].Close(); err != nil {
				return 0, err
			} else {
				n += m.wrts[r].Nbytes()
			}
		}
	}
	return n, nil
}

func (m *Mapper) outputBin() (Bin, error) {
	bin := make(Bin, m.nreducetask)
	outDirPath := MapIntermediateDir(m.job, m.intOutput)
	start := time.Now()
	var pn string
	if strings.Contains(outDirPath, "/s3/") {
		pn = outDirPath
	} else {
		var err error
		pn, err = m.ResolveMounts(outDirPath)
		db.DPrintf(db.MR, "Mapper informReducer ResolveMounts time: %v", time.Since(start))
		if err != nil {
			return nil, fmt.Errorf("%v: ResolveMount %v err %v\n", m.ProcEnv().GetPID(), outDirPath, err)
		}
	}
	for r := 0; r < m.nreducetask; r++ {
		bin[r].File = mshardfile(pn, r) + m.rand
	}
	return bin, nil
}

func (m *Mapper) Emit(key []byte, value string) error {
	if !m.init {
		m.init = true
		// Block if output hasn't been created yet
		if err := <-m.ch; err != nil {
			return err
		}
	}
	r := Khash(key) % m.nreducetask
	var err error
	_, err = encodeKV(m.wrts[r], key, value, r)
	return err
}

// Function for performance debugging
func (m *Mapper) CombineWc(kv *KeyValue) error {
	if _, ok := m.combinewc[kv.Key]; !ok {
		m.combinewc[kv.Key] = 0
	}
	m.combinewc[kv.Key] += 1
	return nil
}

func (m *Mapper) Combine(key []byte, value string) error {
	return m.combined.combine(key, value, m.combinef)
}

func (m *Mapper) CombineEmit() error {
	m.combined.emit(m.combinef, m.Emit)
	m.combined = newKvmap(MINCAP, MAXCAP)
	return nil
}

// Read chunks from the split in parallel
func (m *Mapper) doChunks(i int, pfr *fslib.ParallelFileReader, s *Split, emit EmitT) (sp.Tlength, error) {
	t := sp.Tlength(0)
	for {
		rdr, err := pfr.GetChunkReader(cap(m.bufs[i]))
		if err != nil && err == io.EOF {
			break
		}
		scanner := bufio.NewScanner(rdr)
		scanner.Buffer(m.bufs[i], cap(m.bufs[i]))

		// advance scanner to new line after start, if off != 0
		n := sp.Tlength(0)
		if s.Offset != 0 {
			scanner.Scan()
			l := scanner.Bytes()
			// +1 for newline, but -1 for the extra byte we read (off-- above)
			n += sp.Tlength(len(l))
			db.DPrintf(db.MR, "%v off %v skip %d\n", s.File, s.Offset, n)
		}
		lineRdr := bytes.NewReader([]byte{})
		for scanner.Scan() {
			l := scanner.Bytes()
			n += sp.Tlength(len(l)) + 1 // 1 for newline  XXX or 2 if \r\n
			if len(l) > 0 {
				lineRdr.Reset(l)
				scan := bufio.NewScanner(lineRdr)
				scan.Buffer(m.lines[i], cap(m.lines[i]))
				scan.Split(m.sbc.ScanWords)
				if err := m.mapf(s.File, scan, emit); err != nil {
					return 0, err
				}
			}
			if n >= s.Length {
				db.DPrintf(db.MR, "%v read %v bytes %d extra %d", s.File, n, s.Length, n-s.Length)
				break
			}
		}
		if err := scanner.Err(); err != nil {
			return sp.Tlength(n), err
		}
		t += n
	}
	return t, nil
}

func (m *Mapper) doSplit(s *Split, emit EmitT) (sp.Tlength, error) {
	pn, ok := sp.S3ClientPath(s.File)
	if ok {
		s.File = pn
	}
	db.DPrintf(db.MR, "Mapper doSplit %v\n", s)
	off := s.Offset
	if off != 0 {
		// -1 to pick up last byte from prev split so that if s.Offset
		// != 0 below works out correctly. if the last byte of
		// previous split is a newline, this mapper should process the
		// first line of the split.  if not, this mapper should ignore
		// the first line of the split because it has been processed
		// as part of the previous split.
		off--
	}
	start := time.Now()
	pfr, err := m.OpenParallelFileReader(s.File, s.Offset, s.Length+sp.Tlength(m.linesz))
	if err != nil {
		db.DFatalf("read %v err %v", s.File, err)
	}

	db.DPrintf(db.MR, "Mapper openS3Reader time: %v", time.Since(start))
	defer pfr.Close()

	type result struct {
		n   sp.Tlength
		err error
	}

	ch := make(chan result)
	for i := 0; i < CONCURRENCY; i++ {
		go func(i int) {
			n, err := m.doChunks(i, pfr, s, emit)
			ch <- result{n, err}
		}(i)
	}
	n := sp.Tlength(0)
	for i := 0; i < CONCURRENCY; i++ {
		r := <-ch
		n += r.n
		if r.err != nil {
			return n, err
		}
	}
	return n, nil
}

func (m *Mapper) DoMap() (sp.Tlength, sp.Tlength, Bin, error) {
	db.DPrintf(db.MR, "doMap %v", m.input)
	getInputStart := time.Now()
	var bin Bin
	if err := json.Unmarshal([]byte(m.input), &bin); err != nil {
		db.DPrintf(db.MR, "Mapper %s: unmarshal err %v\n", m.bin, err)
		return 0, 0, nil, err
	}
	emit := m.Emit
	if m.combinef != nil {
		emit = m.Combine
	}
	db.DPrintf(db.TEST, "Mapper getInput time: %v", time.Since(getInputStart))
	ni := sp.Tlength(0)
	getSplitStart := time.Now()
	for _, s := range bin {
		n, err := m.doSplit(&s, emit)
		if err != nil {
			db.DPrintf(db.MR, "doSplit %v err %v\n", s, err)
			return 0, 0, nil, err
		}
		if n < s.Length-1 {
			db.DFatalf("Split: short split o %d l %d %d\n", s.Offset, s.Length, n)
		}
		ni += n
		m.CombineEmit()
	}
	db.DPrintf(db.TEST, "split time: %v", time.Since(getSplitStart))
	closeWrtStart := time.Now()
	nout, err := m.CloseWrt()
	if err != nil {
		return 0, 0, nil, err
	}
	db.DPrintf(db.TEST, "Mapper closeWrt time: %v", time.Since(closeWrtStart))
	obin, err := m.outputBin()
	if err != nil {
		return 0, 0, nil, err
	}
	return ni, nout, obin, nil
}

func RunMapper(mapf MapT, combinef ReduceT, args []string) {
	// debug.SetMemoryLimit(1769 * 1024 * 1024)

	execTimeStr := os.Getenv("SIGMA_EXEC_TIME")
	execTimeMicro, err := strconv.ParseInt(execTimeStr, 10, 64)
	if err != nil {
		db.DFatalf("Error parsing exec time 2: %v", err)
	}
	execTime := time.UnixMicro(execTimeMicro)
	execLat := time.Since(execTime)
	db.DPrintf(db.SPAWN_LAT, "[%v] Proc exec latency: %v", proc.GetSigmaDebugPid(), execLat)
	db.DPrintf(db.ALWAYS, "[%v] Proc exec latency: %v", proc.GetSigmaDebugPid(), execLat)

	init := time.Now()
	pe := proc.GetProcEnv()
	p, err := perf.NewPerf(pe, perf.MRMAPPER)
	if err != nil {
		db.DFatalf("NewPerf err %v\n", err)
	}
	defer p.Done()
	db.DPrintf(db.BENCH, "Mapper [%v] time since spawn: %v", args[2], time.Since(pe.GetSpawnTime()))
	m, err := newMapper(mapf, combinef, args, p)
	if err != nil {
		db.DFatalf("%v: error %v", os.Args[0], err)
	}
	db.DPrintf(db.MR, "Mapper [%v] init time: %v", args[2], time.Since(init))
	start := time.Now()
	nin, nout, outbin, err := m.DoMap()
	db.DPrintf(db.MR_TPT, "%s: in %s out %v tot %v %vms (%s)\n", "map", humanize.Bytes(uint64(nin)), humanize.Bytes(uint64(nout)), test.Mbyte(nin+nout), time.Since(start).Milliseconds(), test.TputStr(nin+nout, time.Since(start).Milliseconds()))
	if err == nil {
		m.ClntExit(proc.NewStatusInfo(proc.StatusOK, "OK",
			Result{true, m.ProcEnv().GetPID().String(), nin, nout, outbin, time.Since(start).Milliseconds(), 0, m.ProcEnv().GetKernelID()}))
	} else {
		m.ClntExit(proc.NewStatusErr(err.Error(), nil))
	}
}

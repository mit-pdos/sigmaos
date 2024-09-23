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
	"sigmaos/writer"
)

const (
	MAXCAP = 32
	MINCAP = 4
)

type Mapper struct {
	*sigmaclnt.SigmaClnt
	mapf        MapT
	combinef    ReduceT
	sbc         *ScanByteCounter
	job         string
	nreducetask int
	linesz      int
	input       string
	intOutput   string
	bin         string
	asyncwrts   []fslib.WriterI
	syncwrts    []*writer.Writer
	pwrts       []*perf.PerfWriter
	rand        string
	perf        *perf.Perf
	asyncrw     bool
	combined    *kvmap
	combinewc   map[string]int
	buf         []byte
	line        []byte
	init        bool
	ch          chan error
}

func NewMapper(sc *sigmaclnt.SigmaClnt, mapf MapT, combinef ReduceT, job string, p *perf.Perf, nr, lsz int, input, intOutput string, asyncrw bool) (*Mapper, error) {
	m := &Mapper{
		mapf:        mapf,
		combinef:    combinef,
		job:         job,
		nreducetask: nr,
		linesz:      lsz,
		rand:        rand.String(16),
		input:       input,
		intOutput:   intOutput,
		bin:         filepath.Base(input),
		asyncwrts:   make([]fslib.WriterI, nr),
		syncwrts:    make([]*writer.Writer, nr),
		pwrts:       make([]*perf.PerfWriter, nr),
		SigmaClnt:   sc,
		perf:        p,
		sbc:         NewScanByteCounter(p),
		asyncrw:     asyncrw,
		combined:    newKvmap(MINCAP, MAXCAP),
		combinewc:   make(map[string]int),
		buf:         make([]byte, 0, lsz),
		line:        make([]byte, 0, lsz),
		ch:          make(chan error),
	}
	go func() {
		m.ch <- m.initOutput()
	}()
	return m, nil
}

func newMapper(mapf MapT, reducef ReduceT, args []string, p *perf.Perf) (*Mapper, error) {
	if len(args) != 6 {
		return nil, fmt.Errorf("NewMapper: too few arguments %v", args)
	}
	nr, err := strconv.Atoi(args[1])
	if err != nil {
		return nil, fmt.Errorf("NewMapper: nreducetask %v isn't int", args[1])
	}
	lsz, err := strconv.Atoi(args[4])
	if err != nil {
		return nil, fmt.Errorf("NewMapper: linesz %v isn't int", args[1])
	}
	start := time.Now()
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.TEST, "NewSigmaClnt done at time: %v", time.Since(start))
	asyncrw, err := strconv.ParseBool(args[5])
	if err != nil {
		return nil, fmt.Errorf("NewMapper: can't parse asyncrw %v", args[5])
	}
	m, err := NewMapper(sc, mapf, reducef, args[0], p, nr, lsz, args[2], args[3], asyncrw)
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
	db.DPrintf(db.MR, "InitWrt %v", name)
	if m.asyncrw {
		if strings.Contains(name, sp.S3) {
			if wrt, err := m.CreateS3AsyncWriter(name, 0777, sp.OWRITE); err != nil {
				return err
			} else {
				m.asyncwrts[r] = wrt
				m.pwrts[r] = perf.NewPerfWriter(wrt, m.perf)
			}
		} else {
			if wrt, err := m.CreateAsyncWriter(name, 0777, sp.OWRITE); err != nil {
				return err
			} else {
				m.asyncwrts[r] = wrt
				m.pwrts[r] = perf.NewPerfWriter(wrt, m.perf)
			}
		}
	} else {
		if wrt, err := m.CreateWriter(name, 0777, sp.OWRITE); err != nil {
			return err
		} else {
			m.syncwrts[r] = wrt
			m.pwrts[r] = perf.NewPerfWriter(wrt, m.perf)
		}
	}

	return nil
}

func (m *Mapper) initOutput() error {
	start := time.Now()
	defer func(start time.Time) {
		db.DPrintf(db.TEST, "initOutput time: %v", time.Since(start))
	}(start)

	// Make a directory for holding the output files of a map task.  Ignore
	// error in case it already exits.  XXX who cleans up?
	m.MkDir(m.intOutput, 0777)
	outDirPath := MapIntermediateOutDir(m.job, m.intOutput, m.bin)
	m.MkDir(filepath.Dir(outDirPath), 0777) // job dir
	m.MkDir(outDirPath, 0777)               // mapper dir
	db.DPrintf(db.MR, "initOutput mkdirs time: %v", time.Since(start))

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
		if m.asyncrw {
			if m.asyncwrts[r] != nil {
				if err := m.asyncwrts[r].Close(); err != nil {
					return 0, err
				} else {
					n += m.asyncwrts[r].Nbytes()
				}
			}
		} else {
			if m.syncwrts[r] != nil {
				if err := m.syncwrts[r].Close(); err != nil {
					return 0, err
				} else {
					n += m.syncwrts[r].Nbytes()
				}
			}
		}
	}
	return n, nil
}

// Inform reducer where to find map output
func (m *Mapper) InformReducer() error {
	outDirPath := MapIntermediateOutDir(m.job, m.intOutput, m.bin)
	pn, err := m.ResolveMounts(outDirPath)
	if err != nil {
		return fmt.Errorf("%v: ResolveMount %v err %v\n", m.ProcEnv().GetPID(), outDirPath, err)
	}
	for r := 0; r < m.nreducetask; r++ {
		fn := mshardfile(pn, r) + m.rand

		name := symname(m.job, strconv.Itoa(r), m.bin)

		// Remove name in case an earlier mapper created the
		// symlink.  A reducer may have opened and is reading
		// the old target, open the new input file and read
		// the new target, or fail because there is no
		// symlink. Failing is fine because the coodinator
		// will start a new reducer once this map completes.
		// We could use rename to atomically remove and create
		// the symlink if we want to avoid the failing case.
		m.Remove(name)

		target := fn + "/"

		db.DPrintf(db.MR, "name %s target %s\n", name, target)

		err = m.Symlink([]byte(target), name, 0777)
		if err != nil {
			db.DFatalf("FATAL symlink %v err %v\n", name, err)
		}
	}
	return nil
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
	if m.asyncrw {
		_, err = encodeKV(m.asyncwrts[r], key, value, r)
	} else {
		_, err = encodeKV(m.syncwrts[r], key, value, r)
	}
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

func (m *Mapper) doSplit(s *Split, emit EmitT) (sp.Tlength, error) {
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
	rdr, err := m.OpenS3AsyncReader(s.File, s.Offset)
	if err != nil {
		db.DFatalf("read %v err %v", s.File, err)
	}
	defer rdr.Close()
	scanner := bufio.NewScanner(rdr)
	scanner.Buffer(m.buf, cap(m.buf))

	// advance scanner to new line after start, if start != 0
	n := 0
	if s.Offset != 0 {
		scanner.Scan()
		l := scanner.Bytes()
		n += len(l) // +1 for newline, but -1 for extra byte we read
	}
	lineRdr := bytes.NewReader([]byte{})
	for scanner.Scan() {
		l := scanner.Bytes()
		n += len(l) + 1 // 1 for newline
		if len(l) > 0 {
			lineRdr.Reset(l)
			scan := bufio.NewScanner(lineRdr)
			scan.Buffer(m.line, cap(m.line))
			scan.Split(m.sbc.ScanWords)
			if err := m.mapf(m.input, scan, emit); err != nil {
				return 0, err
			}
		}
		if sp.Tlength(n) >= s.Length {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return sp.Tlength(n), err
	}
	return sp.Tlength(n), nil
}

func (m *Mapper) DoMap() (sp.Tlength, sp.Tlength, error) {
	db.DPrintf(db.MR, "doMap %v", m.input)
	getInputStart := time.Now()
	rdr, err := m.OpenReader(m.input)
	if err != nil {
		return 0, 0, err
	}
	dec := json.NewDecoder(rdr.GetReader())
	ni := sp.Tlength(0)
	var bin Bin
	if err := dec.Decode(&bin); err != nil && err != io.EOF {
		c, _ := m.GetFile(m.input)
		db.DPrintf(db.MR, "Mapper %s: decode %v err %v\n", m.bin, string(c), err)
		return 0, 0, err
	}
	emit := m.Emit
	if m.combinef != nil {
		emit = m.Combine
	}
	db.DPrintf(db.MR, "Mapper getInput time: %v", time.Since(getInputStart))
	for _, s := range bin {
		db.DPrintf(db.MR, "Mapper %s: process split %v\n", m.bin, s)
		n, err := m.doSplit(&s, emit)
		if err != nil {
			db.DPrintf(db.MR, "doSplit %v err %v\n", s, err)
			return 0, 0, err
		}
		if n < s.Length {
			db.DFatalf("Split: short split o %d l %d %d\n", s.Offset, s.Length, n)
		}
		ni += n
		m.CombineEmit()
	}
	closeWrtStart := time.Now()
	nout, err := m.CloseWrt()
	if err != nil {
		return 0, 0, err
	}
	db.DPrintf(db.MR, "Mapper closeWrtStart time: %v", time.Since(closeWrtStart))
	informReducerStart := time.Now()
	if err := m.InformReducer(); err != nil {
		return 0, 0, err
	}
	db.DPrintf(db.MR, "Mapper informReducer time: %v", time.Since(informReducerStart))
	return ni, nout, nil
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
	db.DPrintf(db.BENCH, "Mapper [%v] time since spawn %v", args[2], time.Since(pe.GetSpawnTime()))
	m, err := newMapper(mapf, combinef, args, p)
	if err != nil {
		db.DFatalf("%v: error %v", os.Args[0], err)
	}
	db.DPrintf(db.MR, "Mapper [%v] init time: %v", args[2], time.Since(init))
	start := time.Now()
	nin, nout, err := m.DoMap()
	db.DPrintf(db.MR_TPT, "%s: in %s out %v tot %v %vms (%s)\n", "map", humanize.Bytes(uint64(nin)), humanize.Bytes(uint64(nout)), test.Mbyte(nin+nout), time.Since(start).Milliseconds(), test.TputStr(nin+nout, time.Since(start).Milliseconds()))
	if err == nil {
		m.ClntExit(proc.NewStatusInfo(proc.StatusOK, m.input,
			Result{true, m.input, nin, nout, time.Since(start).Milliseconds(), 0, m.ProcEnv().GetKernelID()}))
	} else {
		m.ClntExit(proc.NewStatusErr(err.Error(), nil))
	}
}

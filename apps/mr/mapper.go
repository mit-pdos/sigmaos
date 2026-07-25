package mr

import (
	"encoding/json"
	"fmt"
	"os"

	// "runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	"sigmaos/apps/mr/chunkreader"
	"sigmaos/apps/mr/mr"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/proxy/getput"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/procclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/crash"
	"sigmaos/util/perf"
	"sigmaos/util/rand"
)

const (
	CONCURRENCY = 5
)

type Mapper struct {
	*sigmaclnt.SigmaClnt
	mapf         mr.MapT
	combinef     mr.ReduceT
	jobRoot      string
	job          string
	nreducetask  int
	linesz       int
	input        string
	intOutput    string
	wrts         []getput.ShardWriter
	pwrts        []*perf.PerfWriter
	rand         string
	perf         *perf.Perf
	asyncrw      bool
	init         bool
	ckrs         []*chunkreader.ChunkReader
	ch           chan error
	useGetPut    bool
	useCosandbox bool
	tailProbeSz  int
	clnts        *getput.Clnts
}

func NewMapper(sc *sigmaclnt.SigmaClnt, mapf mr.MapT, combinef mr.ReduceT, jobRoot, job string, p *perf.Perf, nr, lsz, wsz int, input string, intOutput string, useGetPut, useCosandbox bool, tailprobesz int) (*Mapper, error) {
	m := &Mapper{
		SigmaClnt:    sc,
		mapf:         mapf,
		combinef:     combinef,
		jobRoot:      jobRoot,
		job:          job,
		nreducetask:  nr,
		linesz:       lsz,
		rand:         rand.Name(),
		input:        input,
		intOutput:    intOutput,
		wrts:         make([]getput.ShardWriter, nr),
		pwrts:        make([]*perf.PerfWriter, nr),
		perf:         p,
		ch:           make(chan error),
		ckrs:         make([]*chunkreader.ChunkReader, CONCURRENCY),
		useGetPut:    useGetPut,
		useCosandbox: useCosandbox,
		tailProbeSz:  tailprobesz,
	}
	for i := 0; i < CONCURRENCY; i++ {
		m.ckrs[i] = chunkreader.NewChunkReader(lsz, wsz, combinef, p)
	}
	// Mount the local UX server from the endpoint the coordinator cached for
	// us, so that neither initOutput's MkDir/Create nor the getput RPC
	// channel has to find it through named (see claude-slop/CACHE_EPs.md).
	// Inline rather than in a goroutine: initOutput needs the mount, and the
	// mount replaces work initOutput would otherwise do. Best-effort — on
	// failure we walk the namespace as before.
	if strings.HasPrefix(m.intOutput, sp.UX) {
		start := time.Now()
		if ok, err := procclnt.MountCachedLocalSrv(sc.FsLib, sp.UX); err != nil {
			db.DPrintf(db.MR, "Mapper MountCachedLocalSrv %v err %v", sp.UX, err)
		} else if ok {
			perf.LogSpawnLatency("Mapper.MountCachedLocalSrv", sc.ProcEnv().GetPID(), sc.ProcEnv().GetSpawnTime(), start)
		}
	}
	if m.useGetPut {
		m.clnts = getput.NewClnts(sc.FsLib)
	}
	m.MountS3PathClnt()
	go func() {
		m.ch <- m.initOutput()
	}()
	return m, nil
}

func newMapper(mapf mr.MapT, reducef mr.ReduceT, args []string, p *perf.Perf) (*Mapper, error) {
	if len(args) != 10 {
		return nil, fmt.Errorf("NewMapper: wrong number of arguments: got %d, want 10 (stale mr-m binary?): %v", len(args), args)
	}
	nr, err := strconv.Atoi(args[2])
	if err != nil {
		return nil, fmt.Errorf("NewMapper: nreducetask %v isn't int", args[2])
	}
	lsz, err := strconv.Atoi(args[5])
	if err != nil {
		return nil, fmt.Errorf("NewMapper: linesz %v isn't int", args[5])
	}
	wsz, err := strconv.Atoi(args[6])
	if err != nil {
		return nil, fmt.Errorf("NewMapper: wordsz %v isn't int", args[6])
	}
	useGetPut, err := strconv.ParseBool(args[7])
	if err != nil {
		return nil, fmt.Errorf("NewMapper: useGetPut %v isn't bool", args[7])
	}
	useCosandbox, err := strconv.ParseBool(args[8])
	if err != nil {
		return nil, fmt.Errorf("NewMapper: useCosandbox %v isn't bool", args[8])
	}
	tailprobesz, err := strconv.Atoi(args[9])
	if err != nil {
		return nil, fmt.Errorf("NewMapper: tailprobesz %v isn't int", args[9])
	}
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return nil, err
	}
	m, err := NewMapper(sc, mapf, reducef, args[0], args[1], p, nr, lsz, wsz, args[3], args[4], useGetPut, useCosandbox, tailprobesz)
	if err != nil {
		return nil, fmt.Errorf("NewMapper failed %v", err)
	}

	if err := m.Started(); err != nil {
		return nil, fmt.Errorf("NewMapper couldn't start %v", args)
	}

	crash.FailersDefault(m.FsLib, []crash.Tselector{crash.MRMAP_CRASH, crash.MRMAP_PARTITION})
	return m, nil
}

func (m *Mapper) CloseWrt() (sp.Tlength, error) {
	// Join the initOutput goroutine before closing the writers and
	// reporting shard names. Only Emit waits for initOutput, so a mapper
	// that emitted nothing (e.g., grep with no matches) would otherwise
	// report its shard names and exit while initOutput is still creating
	// the shard files: the proc's exit detaches its sessions, killing the
	// in-flight creates, and a reducer later fails to read the promised
	// (never-created) shards and exits with RESTART. See
	// claude-slop/MR_REDUCER_UX_BUG.md.
	if !m.init {
		m.init = true
		if err := <-m.ch; err != nil {
			return 0, err
		}
	}
	nout, err := m.closewrts()
	if err != nil {
		return 0, err
	}
	return nout, nil
}

func (m *Mapper) initWrt(r int, name string) error {
	if m.useGetPut {
		db.DPrintf(db.MR, "InitWrt (getput) %v", name)
		wrt, err := getput.NewGetPutWriter(m.clnts, name)
		if err != nil {
			return err
		}
		m.wrts[r] = wrt
		m.pwrts[r] = perf.NewPerfWriter(wrt, m.perf)
		return nil
	}
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
		perf.LogSpawnLatency("Mapper.initOutput", m.ProcEnv().GetPID(), m.ProcEnv().GetSpawnTime(), start)
	}(start)

	if err := CreateMapperIntOutDirUx(m.FsLib, m.job, m.intOutput); err != nil {
		return err
	}
	perf.LogSpawnLatency("Mapper.CreateMapperIntOutDirUx", m.ProcEnv().GetPID(), m.ProcEnv().GetSpawnTime(), start)

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
		// Not covered by the cached-EP mount: resolveMount reads the endpoint
		// file from named to check locality. See claude-slop/CACHE_EPs.md.
		perf.LogSpawnLatency("Mapper.outputBin.ResolveMounts", m.ProcEnv().GetPID(), m.ProcEnv().GetSpawnTime(), start)
		if err != nil {
			return nil, fmt.Errorf("%v: ResolveMount %v err %v\n", m.ProcEnv().GetPID(), outDirPath, err)
		}
	}
	for r := 0; r < m.nreducetask; r++ {
		bin[r].File = mshardfile(pn, r) + m.rand
	}
	return bin, nil
}

// Emit cannot be called in parallel
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

func (m *Mapper) combineEmit() error {
	s := time.Now()
	d := m.ckrs[0]
	for _, ckr := range m.ckrs[1:] {
		d.MergeKVMap(ckr)
	}
	perf.LogSpawnLatency("Mapper.combineEmit", m.ProcEnv().GetPID(), m.ProcEnv().GetSpawnTime(), s)
	err := d.CombineEmit(m.Emit)
	for _, ckr := range m.ckrs {
		ckr.Reset()
	}
	return err
}

// doSplit processes the idx'th split of the mapper's bin. idx doubles as
// the delegated-RPC index when a cosandbox prefetched the bin: the
// coordinator's manifest issues one ranged get per split, in bin order.
func (m *Mapper) doSplit(s *mr.Split, idx int) (sp.Tlength, error) {
	db.DPrintf(db.MR, "Mapper doSplit %v (idx %d)\n", s, idx)
	start := time.Now()
	var pfr getput.SplitReader
	var err error
	if m.useGetPut {
		// The read window MUST match the coordinator's cosandbox manifest
		// (mr.SplitReadWindow); the tail past the probe is fetched lazily
		// with direct (non-delegated) RPCs.
		off, body, probe := mr.SplitReadWindow(s, m.linesz, m.tailProbeSz)
		pfr, err = getput.NewGetPutReader(m.clnts, s.File, off, body, sp.Tlength(m.linesz), probe, m.useCosandbox, uint64(idx))
	} else {
		if pn, ok := sp.S3ClientPath(s.File); ok {
			s.File = pn
		}
		// The offset starts one byte early (when s.Offset != 0) to pick up
		// the last byte of the previous split, so the first chunk can
		// detect — and skip — a leading partial line (see
		// chunkreader.DoChunk). Chunks are generated over [off, splitEnd)
		// only; the final chunk may read up to linesz past the split end —
		// through the first newline — so the line straddling the split end
		// can be completed (the next split's mapper skips it).
		off := s.Offset
		if off != 0 {
			off--
		}
		splitEnd := s.Offset + sp.Toffset(s.Length)
		pfr, err = m.OpenParallelFileReaderSlack(s.File, off, sp.Tlength(splitEnd-off), sp.Tlength(m.linesz))
	}
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
	for _, ckr := range m.ckrs {
		go func(ckr *chunkreader.ChunkReader) {
			n, err := ckr.ReadChunks(pfr, s, m.mapf)
			ch <- result{n, err}
		}(ckr)
	}
	n := sp.Tlength(0)
	for range m.ckrs {
		r := <-ch
		n += r.n
		if r.err != nil {
			return n, err
		}
	}
	if n >= s.Length {
		db.DPrintf(db.MR, "%v read %v bytes %d extra %d", s.File, n, s.Length, n-s.Length)
	}
	err = m.combineEmit()
	return n, err
}

func (m *Mapper) DoMap() (sp.Tlength, sp.Tlength, Bin, error) {
	db.DPrintf(db.MR, "doMap %v", m.input)
	getInputStart := time.Now()
	var bin Bin
	if err := json.Unmarshal([]byte(m.input), &bin); err != nil {
		db.DPrintf(db.MR, "Mapper: unmarshal err %v\n", err)
		return 0, 0, nil, err
	}
	perf.LogSpawnLatency("Mapper.getInput", m.ProcEnv().GetPID(), m.ProcEnv().GetSpawnTime(), getInputStart)
	ni := sp.Tlength(0)
	getSplitStart := time.Now()
	for i, s := range bin {
		n, err := m.doSplit(&s, i)
		if err != nil {
			db.DPrintf(db.MR, "doSplit %v err %v\n", s, err)
			return 0, 0, nil, err
		}
		if n < s.Length-1 {
			db.DFatalf("Split: short split o %d l %d %d\n", s.Offset, s.Length, n)
		}
		ni += n
	}
	perf.LogSpawnLatency("Mapper.doSplit", m.ProcEnv().GetPID(), m.ProcEnv().GetSpawnTime(), getSplitStart)
	closeWrtStart := time.Now()
	nout, err := m.CloseWrt()
	if err != nil {
		return 0, 0, nil, err
	}
	perf.LogSpawnLatency("Mapper.closeWrt", m.ProcEnv().GetPID(), m.ProcEnv().GetSpawnTime(), closeWrtStart)
	obin, err := m.outputBin()
	if err != nil {
		return 0, 0, nil, err
	}
	return ni, nout, obin, nil
}

func RunMapper(mapf mr.MapT, combinef mr.ReduceT, args []string) {
	// debug.SetMemoryLimit(1769 * 1024 * 1024)

	execTimeStr := os.Getenv("SIGMA_EXEC_TIME")
	execTimeMicro, err := strconv.ParseInt(execTimeStr, 10, 64)
	if err != nil {
		db.DFatalf("Error parsing exec time 2: %v", err)
	}
	execTime := time.UnixMicro(execTimeMicro)
	pe := proc.GetProcEnv()
	perf.LogSpawnLatency("Mapper.Exec", pe.GetPID(), pe.GetSpawnTime(), execTime)
	db.DPrintf(db.ALWAYS, "[%v] Proc exec latency: %v", proc.GetSigmaDebugPid(), time.Since(execTime))

	init := time.Now()
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

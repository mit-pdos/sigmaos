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
	"sigmaos/util/crash"
	"sigmaos/util/perf"
	"sigmaos/util/rand"
	"sigmaos/util/tput"
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
	bin          Bin
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
	// Attributes this proc's CPU to its phases; see perf.CPUPhases.
	cpu *perf.CPUPhases
}

// cpu, if non-nil, is the proc's CPU-phase chain (see perf.CPUPhases); the
// mapper marks its setup steps on it. Callers that don't care (tests) pass nil.
func NewMapper(sc *sigmaclnt.SigmaClnt, mapf mr.MapT, combinef mr.ReduceT, jobRoot, job string, p *perf.Perf, nr, lsz, wsz int, input string, intOutput string, useGetPut, useCosandbox bool, tailprobesz int, cpu *perf.CPUPhases) (*Mapper, error) {
	// Decode the input bin up front: DoMap works from it, and so does the
	// decision of which servers to mount below.
	getInputStart := time.Now()
	var bin Bin
	if err := json.Unmarshal([]byte(input), &bin); err != nil {
		db.DPrintf(db.MR, "Mapper: unmarshal %v err %v", input, err)
		return nil, err
	}
	perf.LogSpawnLatency("Mapper.getInput", sc.ProcEnv().GetPID(), sc.ProcEnv().GetSpawnTime(), getInputStart)
	m := &Mapper{
		SigmaClnt:    sc,
		mapf:         mapf,
		combinef:     combinef,
		jobRoot:      jobRoot,
		job:          job,
		nreducetask:  nr,
		linesz:       lsz,
		rand:         rand.Name(),
		bin:          bin,
		intOutput:    intOutput,
		wrts:         make([]getput.ShardWriter, nr),
		pwrts:        make([]*perf.PerfWriter, nr),
		perf:         p,
		ch:           make(chan error),
		ckrs:         make([]*chunkreader.ChunkReader, CONCURRENCY),
		useGetPut:    useGetPut,
		useCosandbox: useCosandbox,
		tailProbeSz:  tailprobesz,
		cpu:          cpu,
	}
	for i := 0; i < CONCURRENCY; i++ {
		m.ckrs[i] = chunkreader.NewChunkReader(lsz, wsz, combinef, p)
	}
	m.mountLocalSrvs()
	// Decoding the input bin, building the chunk readers, and mounting the
	// local UX/S3 servers from the coordinator's cached endpoints.
	m.cpu.Mark("Mapper.mountLocalSrvs")
	if m.useGetPut {
		m.clnts = getput.NewClnts(sc.FsLib)
	}
	// Constructing the S3 path client (an AWS SDK client: config, credential
	// and endpoint resolution, HTTP setup) — done unconditionally, even for a
	// job whose input and output are both in UX.
	m.MountS3PathClnt()
	m.cpu.Mark("Mapper.MountS3PathClnt")
	go func() {
		// initOutput runs concurrently with the phases below, so its CPU is
		// reported on its own rather than as a window in the chain (where it
		// would double-count).
		startCPU := perf.CPUNow()
		err := m.initOutput()
		perf.LogCPUSince("Mapper.initOutput", sc.ProcEnv().GetPID(), sc.ProcEnv().GetSpawnTime(), startCPU)
		m.ch <- err
	}()
	return m, nil
}

// Mount the local instance of each service this mapper will actually touch,
// from the endpoint the coordinator cached for it, so that neither
// initOutput's MkDir/Create nor the getput RPC channels have to find the
// server through named. Inline rather than in a goroutine: initOutput needs
// the mount, and the mount replaces work initOutput would otherwise do.
// Best-effort — a service we can't mount here is found by walking, as before.
func (m *Mapper) mountLocalSrvs() {
	for _, unionpn := range m.srvsUsed() {
		start := time.Now()
		if ok, err := procclnt.MountCachedLocalSrv(m.FsLib, unionpn); err != nil {
			db.DPrintf(db.MR, "Mapper MountCachedLocalSrv %v err %v", unionpn, err)
		} else if ok {
			perf.LogSpawnLatency("Mapper.MountCachedLocalSrv."+unionpn, m.ProcEnv().GetPID(), m.ProcEnv().GetSpawnTime(), start)
		}
	}
}

// srvsUsed reports the union directories of the services this mapper reaches
// through a server it could mount: the one holding its intermediate output,
// and the one(s) holding its input splits. A mapper whose input and output are
// both in UX has no reason to mount an S3 server, and vice versa.
func (m *Mapper) srvsUsed() []string {
	srvs := make([]string, 0, 2)
	if m.usesSrv(sp.UX) {
		srvs = append(srvs, sp.UX)
	}
	// S3 is only reached through a mountable server (the local S3 proxy) on
	// the getput path. Otherwise sp.S3ClientPath rewrites name/s3/~local to
	// the s3clnt path client, which talks to S3 directly.
	if m.useGetPut && m.usesSrv(sp.S3) {
		srvs = append(srvs, sp.S3)
	}
	return srvs
}

func (m *Mapper) usesSrv(unionpn string) bool {
	if strings.HasPrefix(m.intOutput, unionpn) {
		return true
	}
	for _, s := range m.bin {
		if strings.HasPrefix(s.File, unionpn) {
			return true
		}
	}
	return false
}

func newMapper(mapf mr.MapT, reducef mr.ReduceT, args []string, p *perf.Perf, cpu *perf.CPUPhases) (*Mapper, error) {
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
	// Building the SigmaClnt: parsing the ProcEnv, and mounting named and
	// msched (in parallel) from the endpoints procd cached for us.
	cpu.Mark("Mapper.NewSigmaClnt")
	m, err := NewMapper(sc, mapf, reducef, args[0], args[1], p, nr, lsz, wsz, args[3], args[4], useGetPut, useCosandbox, tailprobesz, cpu)
	if err != nil {
		return nil, fmt.Errorf("NewMapper failed %v", err)
	}

	if err := m.Started(); err != nil {
		return nil, fmt.Errorf("NewMapper couldn't start %v", args)
	}
	// Notifying msched that we started.
	cpu.Mark("Mapper.Started")

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
	// (never-created) shards and exits with RESTART.
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

	// Job preparation creates the intermediate output directory on every UX
	// server before any mapper runs (PrepareJob -> CreateIntOutDirsUx), so go
	// straight to creating the shards: checking for the directory here would
	// cost two namespace round trips per mapper to learn it already exists.
	if err := m.initWrts(); err == nil {
		return nil
	}
	// Fall back to creating it: this UX server may not have been covered (e.g.
	// it joined after the job started), or the shard create failed for an
	// unrelated reason, in which case the retry fails the same way.
	if err := CreateMapperIntOutDirUx(m.FsLib, m.job, m.intOutput); err != nil {
		return err
	}
	perf.LogSpawnLatency("Mapper.CreateMapperIntOutDirUx", m.ProcEnv().GetPID(), m.ProcEnv().GetSpawnTime(), start)
	return m.initWrts()
}

// initWrts creates this mapper's output shard, one per reducer. On failure it
// closes whatever it managed to create, so it is safe to call again.
func (m *Mapper) initWrts() error {
	outDirPath := MapIntermediateDir(m.job, m.intOutput)
	for r := 0; r < m.nreducetask; r++ {
		// create temp output shard for reducer r
		oname := mshardfile(outDirPath, r) + m.rand
		if err := m.initWrt(r, oname); err != nil {
			db.DPrintf(db.MR, "initWrt %v err %v", oname, err)
			m.closewrts()
			return err
		}
	}
	return nil
}

// closewrts closes the shard writers created so far and forgets them, so that
// it is safe to call twice — initWrts closes a partial set before its caller
// retries, and CloseWrt closes the final set.
func (m *Mapper) closewrts() (sp.Tlength, error) {
	n := sp.Tlength(0)
	for r := 0; r < m.nreducetask; r++ {
		if m.wrts[r] != nil {
			wrt := m.wrts[r]
			m.wrts[r] = nil
			if err := wrt.Close(); err != nil {
				return 0, err
			} else {
				n += wrt.Nbytes()
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
		// Not covered by the mount we installed at startup — resolveMount
		// reads the endpoint file at name/ux/<kid>, which lives in named, and
		// reading that pathname deliberately bypasses a mount installed there
		// — but served from the coordinator's cached endpoint instead (see
		// fslib.resolveMount).
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
	db.DPrintf(db.MR, "doMap %v", m.bin)
	ni := sp.Tlength(0)
	getSplitStart := time.Now()
	for i, s := range m.bin {
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
	// Reading input, mapping it, and combining — plus whatever the initOutput
	// goroutine does while the first splits run.
	m.cpu.Mark("Mapper.doSplit")
	closeWrtStart := time.Now()
	nout, err := m.CloseWrt()
	if err != nil {
		return 0, 0, nil, err
	}
	perf.LogSpawnLatency("Mapper.closeWrt", m.ProcEnv().GetPID(), m.ProcEnv().GetSpawnTime(), closeWrtStart)
	// Joins initOutput (so any of it that hadn't run yet lands here) and closes
	// the output writers, flushing the shards.
	m.cpu.Mark("Mapper.closeWrt")
	obin, err := m.outputBin()
	if err != nil {
		return 0, 0, nil, err
	}
	m.cpu.Mark("Mapper.outputBin")
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

	// Partition this proc's CPU across its phases, so that the ~20% of it that
	// goes into getting to main (Setup.RuntimeInit.CPU) can be read against
	// where the rest goes. The windows are consecutive, so together with
	// Setup.RuntimeInit.CPU they should account for Proc.exit.CPU.
	cpu := perf.NewCPUPhases(pe.GetPID(), pe.GetSpawnTime())

	init := time.Now()
	p, err := perf.NewPerf(pe, perf.MRMAPPER)
	if err != nil {
		db.DFatalf("NewPerf err %v\n", err)
	}
	defer p.Done()
	db.DPrintf(db.BENCH, "Mapper [%v] time since spawn: %v", args[2], time.Since(pe.GetSpawnTime()))
	m, err := newMapper(mapf, combinef, args, p, cpu)
	if err != nil {
		db.DFatalf("%v: error %v", os.Args[0], err)
	}
	// Whatever setup is left after Started: installing the crash failers.
	m.cpu.Mark("Mapper.setupTail")
	db.DPrintf(db.MR, "Mapper [%v] init time: %v", args[2], time.Since(init))
	start := time.Now()
	nin, nout, outbin, err := m.DoMap()
	db.DPrintf(db.MR_TPT, "%s: in %s out %v tot %v %vms (%s)\n", "map", humanize.Bytes(uint64(nin)), humanize.Bytes(uint64(nout)), tput.Mbyte(nin+nout), time.Since(start).Milliseconds(), tput.TputStr(nin+nout, time.Since(start).Milliseconds()))
	// Whatever is left between DoMap returning and ClntExit (which reports
	// Proc.exit.CPU, i.e. the total).
	m.cpu.Mark("Mapper.postDoMap")
	if err == nil {
		m.ClntExit(proc.NewStatusInfo(proc.StatusOK, "OK",
			Result{true, m.ProcEnv().GetPID().String(), nin, nout, outbin, time.Since(start).Milliseconds(), 0, m.ProcEnv().GetKernelID()}))
	} else {
		m.ClntExit(proc.NewStatusErr(err.Error(), nil))
	}
}

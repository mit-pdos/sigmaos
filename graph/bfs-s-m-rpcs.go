package graph

import (
	"encoding/json"
	"fmt"
	"path"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/graph/proto"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
	"sigmaos/protdevsrv"
	"sigmaos/rand"
	"sigmaos/test"
	"strconv"
)

type BfsMultiMain struct {
	t thread
}

type BfsMultiThread struct {
	t        thread
	g        *GraphPartition
	parents  map[int64]int64
	NS       chan int
	FS       chan int
	pdcs     []*protdevclnt.ProtDevClnt
	jobs     []string
	threadID int
}

func StartBfsMultiMain(public bool, jobname string) error {
	var err error
	b := &BfsMultiMain{}
	if b.t, err = initThread(jobname); err != nil {
		return err
	}
	pds, err := protdevsrv.MakeProtDevSrvPublic(b.t.serverPath, b, public)
	if err != nil {
		db.DPrintf(DEBUG_GRAPH, "|%v| Failed to make ProtDevSrv: %v", b.t.job, err)
		return err
	}
	return pds.RunServer()
}

func (b *BfsMultiMain) RunBfsMulti(ctx fs.CtxI, req proto.BfsIn, res *proto.BfsPath) error {
	jobs := make([]string, MAX_THREADS)
	for j := range jobs {
		jobs[j] = fmt.Sprintf("multi-%v-%v", j, rand.String(8))
	}
	pids := make([]proc.Tpid, MAX_THREADS)
	for i := 0; i < MAX_THREADS; i++ {
		// XXX Write partition to S3 and then read it back
		// XXX ADD threadpaths
		args := append([]string{strconv.FormatBool(test.Overlays), strconv.Itoa(i)}, jobs...)
		p := proc.MakeProc("graph-thread-multi", args)
		p.SetNcore(proc.Tcore(1))
		// Spawn in parallel, wait later
		if err := b.t.Spawn(p); err != nil {
			db.DFatalf("|%v| Error spawning proc %v: %v", b.t.job, p, err)
			return err
		}
		pids[i] = p.GetPid()
	}
	for _, pid := range pids {
		if err := b.t.WaitStart(pid); err != nil {
			db.DFatalf("|%v| Error waiting for proc %v to start: %v", b.t.job, pid, err)
			return err
		}
	}
	// Run after all procs have been initialized
	in := proto.ThreadIn{N2: req.N2}
	outs := make([]proto.ThreadOut, MAX_THREADS)
	for i := range outs {
		outs[i] = proto.ThreadOut{}
	}
	for i := 0; i < MAX_THREADS; i++ {
		pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{b.t.FsLib}, path.Join(DIR_GRAPH, jobs[i], "server"))
		if err != nil {
			return err
		}
		// XXX Is there a way to get rid of this goroutine?
		go func() {
			if err = pdc.RPC("BfsMultiThread.RunBfsMultiThread", &in, &outs[i]); err != nil {
				db.DFatalf("|%v| Error running BFS thread proc: %v", b.t.job, err)
				// XXX Cannot have return val here?
			}
		}()
	}

	return nil
}

func StartBfsMultiThread(public bool, threadID int, jobs []string) error {
	var err error
	t := &BfsMultiThread{}
	if t.t, err = initThread(jobs[threadID]); err != nil {
		return err
	}

	data, err := t.t.GetFile(path.Join(NAMED_GRAPH_DATA, "part-"+strconv.Itoa(threadID)))
	if err != nil {
		return err
	}
	t.g = &GraphPartition{}
	if err = json.Unmarshal(data, t.g); err != nil {
		return err
	}

	t.threadID = threadID
	t.jobs = jobs
	t.parents = make(map[int64]int64, t.g.NumNodes)
	t.NS = make(chan int, 0)
	t.FS = make(chan int, 0)
	t.pdcs = make([]*protdevclnt.ProtDevClnt, len(t.jobs))

	pds, err := protdevsrv.MakeProtDevSrvPublic(t.t.serverPath, t, public)
	if err != nil {
		db.DPrintf(DEBUG_GRAPH, "|%v| Failed to make ProtDevSrv: %v", t.t.job, err)
		return err
	}
	db.DPrintf(DEBUG_GRAPH, "Created New BFS Multi Thread: %v", t)
	return pds.RunServer()
}

func (t *BfsMultiThread) RunBfsMultiThread(ctx fs.CtxI, req proto.ThreadIn, res *proto.ThreadOut) error {
	var err error
	// Done here because the servers need to be running first
	for i, job := range t.jobs {
		if t.pdcs[i], err = protdevclnt.MkProtDevClnt([]*fslib.FsLib{t.t.FsLib}, path.Join(DIR_GRAPH, job, "server")); err != nil {
			return err
		}
	}
	db.DPrintf(DEBUG_GRAPH, "RUNNING BFS MULTI THREAD")

	return nil
}

func (t *BfsMultiThread) Put(ctx fs.CtxI, req proto.Index, res *proto.None) error {
	t.NS <- int(req.Val)
	return nil
}

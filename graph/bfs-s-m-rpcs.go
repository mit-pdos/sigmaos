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
	"sync"
)

type BfsMultiMain struct {
	t thread
}

type BfsMultiThread struct {
	t        thread
	g        *GraphPartition
	parents  map[int64]int64
	NS       chan pair64
	FS       chan int64
	done     chan bool
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
	wg := sync.WaitGroup{}
	for i := 0; i < MAX_THREADS; i++ {
		pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{b.t.FsLib}, path.Join(DIR_GRAPH, jobs[i], "server"))
		if err != nil {
			return err
		}
		if getOwner(int(req.N1), MAX_THREADS) == i {
			inPair := proto.Pair{Child: req.N1, Parent: req.N1}
			x := proto.None{}
			if err = pdc.RPC("BfsMultiThread.PutPair", &inPair, &x); err != nil {
				db.DFatalf("|%v| Error running initial PutPair for BFS Multithreaded: %v", b.t.job, err)
			}
		}
		// XXX Is there a way to get rid of this goroutine?
		wg.Add(1)
		j := i
		go func() {
			if err = pdc.RPC("BfsMultiThread.RunBfsMultiThread", &in, &outs[j]); err != nil {
				db.DFatalf("|%v| Error running BFS thread proc: %v", b.t.job, err)
				// XXX Find a way to return err here
				panic("RunBfsMultiThread Failed")
			}
			wg.Done()
		}()
	}
	wg.Wait()
	parents := make([]map[int64]int64, MAX_THREADS)
	for i := range outs {
		parents[i] = outs[i].ParentPartition
	}
	res.Val = *findPathPartitioned64(&parents, req.N1, req.N2)
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
	t.NS = make(chan pair64, t.g.NumEdges)
	t.FS = make(chan int64, t.g.NumEdges)
	t.done = make(chan bool, 1)
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
	// db.DPrintf(DEBUG_GRAPH, "Running BFS Multi Thread")
	for key := range t.g.N {
		t.parents[int64(key)] = -1
	}
	for {
		select {
		case <-t.done:
			res.ParentPartition = t.parents
			return nil
		case index := <-t.NS:
			if t.parents[index.child] == -1 {
				// db.DPrintf(DEBUG_GRAPH, "|%v| Processing NSs %v", t.threadID, index)
				t.parents[index.child] = index.parent
				if index.child == req.N2 {
					// db.DPrintf(DEBUG_GRAPH, "|%v| Found solution %v from parent %v", t.threadID, index.child, index.parent)
					for _, pdc := range t.pdcs {
						x := proto.None{}
						if err = pdc.RPC("BfsMultiThread.Done", &x, &x); err != nil {
							return err
						}
					}
				}
				t.FS <- index.child
			}
		case index := <-t.FS:
			// db.DPrintf(DEBUG_GRAPH, "|%v| Processing FSs %v", t.threadID, index)
			adj := t.g.getNeighbors(int(index))
			for _, a := range adj {
				in := proto.Pair{Child: int64(a), Parent: index}
				out := proto.None{}
				if err = t.pdcs[getOwner(a, MAX_THREADS)].RPC("BfsMultiThread.PutPair", &in, &out); err != nil {
					return err
				}
			}
		}
	}
}

func (t *BfsMultiThread) PutPair(ctx fs.CtxI, req proto.Pair, res *proto.None) error {
	t.NS <- pair64{req.Child, req.Parent}
	return nil
}

func (t *BfsMultiThread) Done(ctx fs.CtxI, req proto.None, res *proto.None) error {
	t.done <- true
	return nil
}

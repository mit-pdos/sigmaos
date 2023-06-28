package graph

import (
	"path"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/graph/proto"
	"sigmaos/perf"
	"sigmaos/protdevclnt"
	"sigmaos/protdevsrv"
)

type BfsSingle struct {
	t       thread
	g       *Graph
	parents []int
	cs      chan int
	pdc     *protdevclnt.ProtDevClnt
}

func StartThreadSingle(public bool, jobname string) error {
	var err error
	profiler, err := perf.MakePerf(perf.BENCH)
	if err != nil {
		db.DPrintf(DEBUG_GRAPH, "|%v| Failed to MakePerf: %v", jobname, err)
		return err
	}
	defer profiler.Done()
	b := &BfsSingle{}
	if b.t, err = initThread(jobname); err != nil {
		return err
	}

	data, err := b.t.GetFile(path.Join(NAMED_GRAPH_DATA, "full"))
	if err != nil {
		return err
	}
	b.g, err = ImportGraph(string(data))

	// XXX Replace with variable length queue
	b.cs = make(chan int, b.g.NumEdges)
	pds, err := protdevsrv.MakeProtDevSrvPublic(b.t.serverPath, b, public)
	if err != nil {
		db.DPrintf(DEBUG_GRAPH, "|%v| Failed to make ProtDevSrv: %v", b.t.job, err)
		return err
	}
	db.DPrintf(DEBUG_GRAPH, "Created Single Thread: %v", b)
	return pds.RunServer()
}

func (b *BfsSingle) RunBfsSingle(ctx fs.CtxI, req proto.BfsIn, res *proto.BfsPath) error {
	db.DPrintf(DEBUG_GRAPH, "Running BFS Single from %v to %v", req.N1, req.N2)

	n1 := int(req.N1)
	n2 := int(req.N2)
	var err error
	// This is done here instead of in StartThreadSingle because the rpc server must be initialized first.
	if b.pdc, err = protdevclnt.MkProtDevClnt([]*fslib.FsLib{b.t.FsLib}, b.t.serverPath); err != nil {
		return err
	}
	b.parents = make([]int, b.g.NumNodes)
	for i := range b.parents {
		b.parents[i] = NOT_VISITED
	}
	putReq := proto.Index{Val: req.N1}
	putRes := proto.None{}
	b.parents[n1] = n1
	if err := b.pdc.RPC("BfsSingle.Put", &putReq, &putRes); err != nil {
		return err
	}

	// XXX End Condition for No Path
	for {
		index := <-b.cs
		adj := b.g.GetNeighbors(index)
		for _, a := range *adj {
			if b.parents[a] == NOT_VISITED {
				putReq := proto.Index{Val: int64(a)}
				if err := b.pdc.RPC("BfsSingle.Put", &putReq, &putRes); err != nil {
					return err
				}

				b.parents[a] = index
				if a == n2 {
					// Return the shortest path from n1 to n2
					res.Val = *findPath64(&b.parents, n1, n2)
					return nil
				}
			}
		}
	}
	return ERR_NOPATH
}

func (b *BfsSingle) Put(ctx fs.CtxI, req proto.Index, res *proto.None) error {
	b.cs <- int(req.Val)
	return nil
}

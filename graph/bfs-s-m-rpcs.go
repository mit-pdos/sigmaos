package graph

import (
	"encoding/json"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/graph/proto"
	"sigmaos/protdevclnt"
	"sigmaos/protdevsrv"
)

type BfsMultiMain struct {
	t thread
	g *Graph
}

type BfsMultiThread struct {
	t        thread
	g        *graphPartition
	parents  map[int64]int64
	NS       chan int
	FS       chan int
	pdcs     []*protdevclnt.ProtDevClnt
	threadID int
}

func StartBfsMultiMain(public bool, jobname string, graph string) error {
	var err error
	b := BfsMultiMain{}
	if b.t, err = initThread(jobname); err != nil {
		return err
	}
	b.g = &Graph{}
	if err = json.Unmarshal([]byte(graph), b.g); err != nil {
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
	for i := 0; i < MAX_THREADS; i++ {

	}
	return nil
}

func StartBfsMultiThread(public bool, jobname string, partition string, threadID int, threadPaths []string) error {
	var err error
	t := BfsMultiThread{}
	if t.t, err = initThread(jobname); err != nil {
		return err
	}
	t.g = &graphPartition{}
	if err = json.Unmarshal([]byte(partition), t.g); err != nil {
		return err
	}
	t.threadID = threadID
	for i, path := range threadPaths {
		if t.pdcs[i], err = protdevclnt.MkProtDevClnt([]*fslib.FsLib{t.t.FsLib}, path); err != nil {
			return err
		}
	}
	t.parents = make(map[int64]int64, t.g.numNodes)
	t.NS = make(chan int, 0)
	t.FS = make(chan int, 0)
	pds, err := protdevsrv.MakeProtDevSrvPublic(t.t.serverPath, t, public)
	if err != nil {
		db.DPrintf(DEBUG_GRAPH, "|%v| Failed to make ProtDevSrv: %v", t.t.job, err)
		return err
	}
	return pds.RunServer()
}

func (t *BfsMultiThread) RunBfsMultiThread(ctx fs.CtxI, req proto.ThreadIn, res *proto.ThreadOut) error {
	return nil
}

func (t *BfsMultiThread) Put(ctx fs.CtxI, req proto.Index, res *proto.None) error {
	t.NS <- int(req.Val)
	return nil
}

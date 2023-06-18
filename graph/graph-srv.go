package graph

import (
	"encoding/json"
	"path"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/graph/proto"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
	"sigmaos/protdevsrv"
	"sigmaos/rand"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"strconv"
)

const DIR_GRAPH = sp.NAMED + "graph/"
const NAMED_GRAPH_DATA = DIR_GRAPH + "g-data/"
const NAMED_GRAPH_SERVER = DIR_GRAPH + "g-server/"
const GRAPH_DATA_FN = sp.NAMED + "graph-data"

const (
	BFS_SINGLE_RPC = iota + 1
	BFS_MULTI_RPC
)

type GraphSrv struct {
	sc         *sigmaclnt.SigmaClnt
	numNodes   int
	numEdges   int
	job        string
	serverPath string
}

type thread struct {
	*sigmaclnt.SigmaClnt
	job        string
	serverPath string
}

//
//  name
//  ├─graph-data
//  └─graph
//    ├─g-server
//    ├─g-data
//    │ ├─full
//    │ ├─part-1
//    │ ├─part-2
//    │ └─...
//    ├─thread1
//    │ └─server
//    │   └─...
//    ├─thread2
//    │ └─server
//    │   └─...
//    ├─thread3
//    │ └─server
//    │   └─...
//    │...
//
// XXX Add support for multiple graphs running simultaneously
// To have multiple graphs, I'd wrap everything under name/graph/ in
// a new directory for each job.
//
// XXX TODO: Move partition into s3
//

// initGraphNamespace returns the path of the graph's RPC server directory
func initGraphNamespace(fs *fslib.FsLib, job string) (string, error) {
	// XXX Add support for multiple graphs running simultaneously
	db.DPrintf(DEBUG_GRAPH, "|%v| Setting up graph namespace", job)
	var err error
	if err = fs.MkDir(DIR_GRAPH, 0777); err != nil {
		db.DFatalf("|%v| Graph error creating %v directory: %v", job, DIR_GRAPH, err)
		return "", err
	}
	if err = fs.MkDir(NAMED_GRAPH_SERVER, 0777); err != nil {
		db.DFatalf("|%v| Graph error creating %v directory: %v", job, NAMED_GRAPH_SERVER, err)
		return "", err
	}
	if err = fs.MkDir(NAMED_GRAPH_DATA, 0777); err != nil {
		db.DFatalf("|%v| Graph error creating %v directory: %v", job, NAMED_GRAPH_DATA, err)
		return "", err
	}
	return NAMED_GRAPH_SERVER, nil
}

func initThread(job string) (thread, error) {
	thread := thread{}
	thread.job = job
	db.DPrintf(DEBUG_GRAPH, "|%v| Setting up thread namespace", job)
	var err error
	sc, err := sigmaclnt.MkSigmaClnt(job)
	if err != nil {
		return thread, err
	}
	jobPath := path.Join(DIR_GRAPH, job)
	if err = sc.MkDir(jobPath, 0777); err != nil {
		db.DFatalf("|%v| Graph error creating %v directory: %v", job, jobPath, err)
		return thread, err
	}
	// RPC Server
	serverPath := path.Join(jobPath, "server/")
	if err = sc.MkDir(serverPath, 0777); err != nil {
		db.DFatalf("|%v| Graph error creating %v directory: %v", job, serverPath, err)
		return thread, err
	}

	thread.SigmaClnt = sc
	thread.serverPath = serverPath
	return thread, nil
}

func StartGraphSrv(public bool, jobname string) error {
	g := &GraphSrv{}
	g.job = jobname

	// Init Namespace
	// XXX This should not be random
	sc, err := sigmaclnt.MkSigmaClnt(rand.String(8))
	if err != nil {
		return err
	}
	g.sc = sc
	// XXX This should not be random
	graphServer, err := initGraphNamespace(sc.FsLib, rand.String(8))
	if err != nil {
		return err
	}
	g.serverPath = graphServer

	pds, err := protdevsrv.MakeProtDevSrvPublic(graphServer, g, public)
	if err != nil {
		db.DPrintf(DEBUG_GRAPH, "|%v| Failed to make ProtDevSrv: %v", g.job, err)
		return err
	}
	return pds.RunServer()
}

func (g *GraphSrv) ImportGraph(ctx fs.CtxI, req proto.GraphIn, res *proto.GraphOut) error {
	// Import full graph
	data, err := g.sc.GetFile(req.Fn)
	if err != nil {
		return err
	}
	graph, err := ImportGraph(string(data))
	if err != nil {
		return err
	}
	// Copy data in for threads
	_, err = g.sc.PutFile(path.Join(NAMED_GRAPH_DATA, "full"), 0777, sp.OWRITE, data)
	if err != nil {
		return err
	}
	// Write partitions for multithreaded BFS
	graphs := graph.partition(MAX_THREADS)
	for i := range graphs {
		marshaled, err := json.Marshal(graphs[i])
		if err != nil {
			return err
		}
		_, err = g.sc.PutFile(path.Join(NAMED_GRAPH_DATA, "part-"+strconv.Itoa(i)), 0777, sp.OWRITE, marshaled)
		if err != nil {
			return err
		}
	}
	g.numNodes = graph.NumNodes
	g.numEdges = graph.NumEdges
	res.Nodes = int64(graph.NumNodes)
	res.Edges = int64(graph.NumEdges)
	return nil
}

func (g *GraphSrv) RunBfs(ctx fs.CtxI, req proto.BfsInput, res *proto.Path) error {
	var err error

	n1 := int(req.N1)
	n2 := int(req.N2)
	if n1 == n2 {
		res.Marshaled, err = json.Marshal(&[]int64{req.N1})
		return err
	}
	if n1 > g.numNodes-1 || n2 > g.numNodes-1 || n1 < 0 || n2 < 0 {
		return ERR_SEARCH_OOR
	}

	switch req.Alg {
	case BFS_SINGLE_RPC:
		job := "single-" + rand.String(8)
		p := proc.MakeProc("graph-thread-single", []string{strconv.FormatBool(test.Overlays), job})
		p.SetNcore(proc.Tcore(1))
		if err = g.sc.Spawn(p); err != nil {
			db.DFatalf("|%v| Error spawning proc %v: %v", g.job, p, err)
			return err
		}
		if err = g.sc.WaitStart(p.GetPid()); err != nil {
			db.DFatalf("|%v| Error waiting for proc %v to start: %v", g.job, p, err)
			return err
		}
		// XXX Get path from proc
		pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{g.sc.FsLib}, path.Join(DIR_GRAPH, job, "server"))
		if err != nil {
			return err
		}
		bfsReq := proto.BfsIn{N1: req.N1, N2: req.N2}
		bfsRes := proto.BfsPath{}
		if err = pdc.RPC("BfsSingle.RunBfsSingle", &bfsReq, &bfsRes); err != nil {
			db.DFatalf("|%v| Error running BFS proc %v: %v", g.job, p, err)
			return err
		}
		res.Marshaled, err = json.Marshal(bfsRes.Val)
		return err
	case BFS_MULTI_RPC:
		out := make([]int, 0)
		/*if out, err = g.BfsMultiRPC(ctx, int(req.N1), int(req.N2)); err != nil {
			return nil
		}*/
		res.Marshaled, err = json.Marshal(&out)
		return err
	default:
		return mkErr("Invalid BFS Request")
	}
}

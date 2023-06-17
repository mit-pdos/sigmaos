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

const (
	BFS_SINGLE_RPC = iota + 1
	BFS_MULTI_RPC
)

type GraphSrv struct {
	sc         *sigmaclnt.SigmaClnt
	g          *Graph
	job        string
	serverPath string
	// XXX Cache the parents slice
}

type thread struct {
	*sigmaclnt.SigmaClnt
	job        string
	serverPath string
}

//
//  name
//  └─graph
//    ├─g-server
//    ├─job1
//    │ └─server
//    │   └─...
//    ├─job2
//    │ └─server
//    │   └─...
//    ├─job3
//    │ └─server
//    │   └─...
//    │...
//
// To have multiple graphs, I'd wrap everything under name/graph/ in
// a new directory for each job.
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
	jobServer := path.Join(DIR_GRAPH, "g-server/")
	if err = fs.MkDir(jobServer, 0777); err != nil {
		db.DFatalf("|%v| Graph error creating %v directory: %v", job, jobServer, err)
		return "", err
	}
	return jobServer, nil
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
	g.g = &Graph{}

	// Init Namespace
	sc, err := sigmaclnt.MkSigmaClnt(rand.String(8))
	if err != nil {
		return err
	}
	g.sc = sc
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
	// XXX Store graph data in s3 and get based on filename
	g.g = &Graph{}
	if err := json.Unmarshal(req.Marshaled, g.g); err != nil {
		return err
	}
	res.Nodes = int64(g.g.NumNodes)
	res.Edges = int64(g.g.NumEdges)
	return nil
}

func (g *GraphSrv) RunBfs(ctx fs.CtxI, req proto.BfsInput, res *proto.Path) error {
	var err error

	switch req.Alg {
	case BFS_SINGLE_RPC:
		var marshaled []byte
		if marshaled, err = json.Marshal(g.g); err != nil {
			return err
		}
		p := proc.MakeProc("graph-thread-single", []string{strconv.FormatBool(test.Overlays), "single", string(marshaled)})
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
		pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{g.sc.FsLib}, path.Join(DIR_GRAPH, "single", "server"))
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

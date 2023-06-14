package graph

import (
	"encoding/json"
	"path"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/graph/proto"
	"sigmaos/protdevsrv"
	"sigmaos/rand"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const DIR_GRAPH = sp.NAMED + "graph/"

type GraphSrv struct {
	job string
	sc  *sigmaclnt.SigmaClnt
	g   *Graph
	// XXX Store the parents slice for caching
}

type Thread struct {
	*sigmaclnt.SigmaClnt
	Pipe   string
	Server string
}

//
//  name
//  └─graph
//    ├─g-server
//    ├─job1
//    │ ├─pipe
//    │ └─server
//    │   └─...
//    ├─job2
//    │ ├─pipe
//    │ └─server
//    │   └─...
//    ├─job3
//    │ ├─pipe
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

func initThread(job string) (Thread, error) {
	thread := Thread{}
	db.DPrintf(DEBUG_GRAPH, "|%v| Setting up thread namespace", job)
	var err error
	sc, err := sigmaclnt.MkSigmaClnt(rand.String(8))
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
	// Pipe
	pipePath := path.Join(jobPath, "pipe")
	if err = sc.MakePipe(pipePath, 0777); err != nil {
		return thread, err
	}

	thread.SigmaClnt = sc
	thread.Server = serverPath
	thread.Pipe = pipePath
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

	pds, err := protdevsrv.MakeProtDevSrvPublic(graphServer, g, public)
	if err != nil {
		db.DPrintf(DEBUG_GRAPH, "|%v| Failed to make ProtDevSrv: %v", g.job, err)
		return err
	}
	return pds.RunServer()
}

func (g *GraphSrv) ImportGraph(ctx fs.CtxI, req proto.GraphIn, res *proto.GraphOut) error {
	// Make sure it's reset; I'm unsure what the behavior is when json unmarshals into
	// an existing data structure.
	g.g = &Graph{}
	if err := json.Unmarshal(req.Marshaled, g.g); err != nil {
		return err
	}
	res.Nodes = int64(g.g.NumNodes)
	res.Edges = int64(g.g.NumEdges)
	return nil
}

func (g *GraphSrv) RunBfsSinglePipes(ctx fs.CtxI, req proto.BfsInput, res *proto.Path) error {
	// Init
	_, err := initThread(rand.String(8))
	if err != nil {
		return err
	}
	// Don't run the test; something's going wrong with initialization.
	/*db.DPrintf(DEBUG_GRAPH, "Running Test")
	out, err := g.g.BfsSinglePipes(int(req.GetN1()), int(req.GetN2()), thread)
	if IsNoPath(err) {
		db.DPrintf(DEBUG_GRAPH, "No Valid Path from %v to %v in graph of size %v", req.GetN1(), req.GetN2(), g.g.NumNodes)
	} else if err != nil {
		return err
	}

	res.Marshaled, err = json.Marshal(out)*/
	return err
}

package example_echo_server

import (
	dbg "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/maze"
	"sigmaos/protdevsrv"
	"sigmaos/rand"
	sp "sigmaos/sigmap"
)

// YH:
// Toy server echoing request message

type EchoSrv struct {
	sid string
}

const DEBUG_ECHO_SERVER = "ECHO_SERVER"
const DIR_ECHO_SERVER = sp.NAMED + "example/"
const NAMED_ECHO_SERVER = DIR_ECHO_SERVER + "echo-server"

func RunEchoSrv(public bool) error {
	echosrv := &EchoSrv{rand.String(8)}
	dbg.DPrintf(DEBUG_ECHO_SERVER, "==%v== Creating echo server \n", echosrv.sid)
	pds, err := protdevsrv.MakeProtDevSrvPublic(NAMED_ECHO_SERVER, echosrv, public)
	if err != nil {
		return err
	}
	dbg.DPrintf(DEBUG_ECHO_SERVER, "==%v== Starting to run echo service\n", echosrv.sid)
	return pds.RunServer()
}

// find meaning of life for request
// XXX WEIRD ERROR: making Req a pointer causes it to crash.
func (echosrv *EchoSrv) Echo(ctx fs.CtxI, req EchoRequest, rep *EchoResult) error {
	dbg.DPrintf(DEBUG_ECHO_SERVER, "==%v== Received Echo Request: %v\n", echosrv.sid, req)
	mazeReq := maze.MazeRequest{}
	mazeReq.Height = 10
	mazeReq.Width = 10
	mazeReq.Density = 15
	mazeReq.GenerateAlg = maze.GEN_DFS
	mazeReq.SolveAlg = maze.SOLVE_BFS_SINGLE
	mazeRes := maze.MazeResponse{}
	var err error
	if err = maze.GetMaze(&mazeReq, &mazeRes); err != nil {
		return err
	}
	rep.Text = mazeRes.BestPath

	//rep.Text = req.Text
	return nil
}

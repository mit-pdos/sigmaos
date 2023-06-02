package maze

import (
	"fmt"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/protdevsrv"
	"sigmaos/rand"
	sp "sigmaos/sigmap"
)

type SrvMaze struct {
	sid string
}

const DEBUG_MAZE = "MAZE"
const DIR_MAZE = sp.NAMED + "maze/"
const NAMED_MAZE_SERVER = DIR_MAZE + "m-server/"

const minsize = 3
const maxsize = 1000

// XXX I have no idea what the public bool does.
// XXX TODO Problem with creation of protdevsrv; doesn't make clone-rpc.
// - Do I need to initialize some sort of directory?
func RunMaze(public bool) error {
	ms := &SrvMaze{}
	ms.sid = rand.String(8)
	db.DPrintf(DEBUG_MAZE, "|%v| Creating maze server\n", ms.sid)
	pds, err := protdevsrv.MakeProtDevSrvPublic(NAMED_MAZE_SERVER, ms, true)
	if err != nil {
		db.DPrintf(DEBUG_MAZE, "|%v| Failed to make ProtDevSrv: %v", ms.sid, err)
		return err
	}
	return pds.RunServer()
}

func (ms *SrvMaze) Maze(ctx fs.CtxI, req MazeRequest, rep *MazeResponse) error {
	db.DPrintf(DEBUG_MAZE, "|%v| Received Maze Request: %v\n", ms.sid, req)
	return GetMaze(&req, rep)
}

func GetMaze(req *MazeRequest, rep *MazeResponse) error {
	if req == nil {
		return mkErr("invalid request (empty)")
	}
	w, h, d := int(req.GetWidth()), int(req.GetHeight()), int(req.GetDensity())
	if w < minsize || h < minsize || w > maxsize || h > maxsize {
		return mkErr(fmt.Sprintf("invalid maze size: %v", req))
	}

	m, err := makeMaze(w, h, d, req.GetGenerateAlg())
	if err != nil {
		return err
	}

	paths, solution, err := solveMaze(m, req.GetSolveAlg())
	if err != nil {
		return err
	}

	rep.SearchPaths = string(pathsToJs(m, paths))
	rep.BestPath = string(pathToJs(m, solution))
	return nil
}

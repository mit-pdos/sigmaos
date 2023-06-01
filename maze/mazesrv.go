package maze

import (
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/protdevsrv"
	"sigmaos/rand"
	sp "sigmaos/sigmap"
)

type MazeSrv struct {
	sid string
}

const DEBUG_MAZE = "MAZE"
const DIR_MAZE = sp.NAMED + "maze/"
const NAMED_MAZE_SERVER = DIR_MAZE + "maze-server"

// XXX I have no idea what the public bool does.
func RunMaze(public bool) error {
	ms := &MazeSrv{}
	ms.sid = rand.String(8)
	db.DPrintf(DEBUG_MAZE, "|%v| Creating maze server\n", ms.sid)
	pds, err := protdevsrv.MakeProtDevSrvPublic(NAMED_MAZE_SERVER, ms, public)
	if err != nil {
		return err
	}
	return pds.RunServer()
}

func (ms *MazeSrv) SolveMaze(ctx fs.CtxI, req *MazeRequest, rep *MazeResponse) error {
	db.DPrintf(DEBUG_MAZE, "|%v| Received Maze Request: %v\n", ms.sid, req)
	// TODO call maze func
	rep.Maze = "This is the maze response!"
	return nil
}

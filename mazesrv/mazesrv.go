package mazesrv

import (
	"bytes"
	"errors"
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

func mkErr(message string) error {
	return errors.New("MazeSrv: " + message)
}

// XXX I have no idea what the public bool does.
// XXX TODO Problem with creation of protdevsrv; doesn't make clone-rpc.
// - Do I need to initialize some sort of directory?
func RunMaze(public bool) error {
	ms := &SrvMaze{}
	ms.sid = rand.String(8)
	db.DPrintf(DEBUG_MAZE, "|%v| Creating maze server\n", ms.sid)
	pds, err := protdevsrv.MakeProtDevSrvPublic(NAMED_MAZE_SERVER, ms, public)
	if err != nil {
		db.DPrintf(DEBUG_MAZE, "|%v| Failed to make ProtDevSrv: %v", ms.sid, err)
		return err
	}
	return pds.RunServer()
}

func (ms *SrvMaze) Maze(ctx fs.CtxI, req MazeRequest, rep *MazeResponse) error {
	db.DPrintf(DEBUG_MAZE, "|%v| Received Maze Request: %v\n", ms.sid, req)
	// This is weirdly split so that I can call GetMaze from echosrv for
	// debugging while I'm working on fixing the maze proc.
	return GetMaze(&req, rep)
}

func GetMaze(req *MazeRequest, rep *MazeResponse) error {
	if req == nil {
		return mkErr("invalid request (empty)")
	}

	in := MazeInputs{
		width:      int(req.GetWidth()),
		height:     int(req.GetHeight()),
		tickSpeed:  int(req.GetTickSpeed()),
		repeats:    int(req.GetRepeats()),
		density:    int(req.GetDensity()),
		solveAlg:   req.GetSolveAlg(),
		genAlg:     req.GetGenerateAlg(),
		startIndex: int(req.GetStartIndex()),
	}

	buf := new(bytes.Buffer)
	err := makeMaze(&in, buf)
	if err != nil {
		return err
	}

	rep.Webpage = buf.String()
	return nil
}

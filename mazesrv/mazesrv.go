package mazesrv

import (
	"bytes"
	"errors"
	"path"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/protdevsrv"
	"sigmaos/rand"
	sp "sigmaos/sigmap"
)

type SrvMaze struct {
	sid  string
	maze *[]byte
}

const DEBUG_MAZE = "MAZE"
const DIR_MAZE = sp.NAMED + "maze/"
const NAMED_MAZE_SERVER = DIR_MAZE + "m-server/"

func mkErr(message string) error {
	return errors.New("MazeSrv: " + message)
}

// XXX Is this the right place for this function? I feel like it's
// better than in mazesrv_test.go
func InitMazeNamespace(fs *fslib.FsLib, job string) error {
	db.DPrintf(DEBUG_MAZE, "|%v| Setting up maze namespace", job)
	var err error
	jobDir := path.Join(DIR_MAZE, job)
	// Setup working namespace
	if err = fs.MkDir(DIR_MAZE, 0777); err != nil {
		db.DFatalf("|%v| Error setting up the working namespace for Maze when creating %v directory: %v", job, DIR_MAZE, err)
		return err
	}
	if err = fs.MkDir(jobDir, 0777); err != nil {
		db.DFatalf("|%v| Error setting up the working namespace for Maze when creating %v directory: %v", job, jobDir, err)
		return err
	}
	if err = fs.MkDir(NAMED_MAZE_SERVER, 0777); err != nil {
		db.DFatalf("|%v| Error setting up the working namespace for Maze when creating %v directory: %v", job, NAMED_MAZE_SERVER, err)
		return err
	}
	return nil
}

func RunMaze(public bool) error {
	ms := &SrvMaze{}
	ms.sid = rand.String(8)
	db.DPrintf(DEBUG_MAZE, "|%v| Creating maze server\n", ms.sid)
	pds, err := protdevsrv.MakeProtDevSrvPublic(NAMED_MAZE_SERVER, ms, public)
	if err != nil {
		db.DPrintf(DEBUG_MAZE, "|%v| Failed to make ProtDevSrv: %v", ms.sid, err)
		return err
	}
	db.DPrintf(DEBUG_MAZE, "|%v| Generating Maze\n", ms.sid)

	return pds.RunServer()
}

// XXX Why is MazeRequest not a pointer?
func (ms *SrvMaze) GetMaze(ctx fs.CtxI, req MazeRequest, rep *MazeResponse) error {
	db.DPrintf(DEBUG_MAZE, "|%v| Received Maze Request: %v\n", ms.sid, req)

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
	err := makeSolveMaze(&in, buf)
	if err != nil {
		return err
	}

	rep.Webpage = buf.String()
	return nil
}

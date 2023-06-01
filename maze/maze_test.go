package maze_test

import (
	"github.com/stretchr/testify/assert"
	"path"
	db "sigmaos/debug"
	"sigmaos/fslib"
	maze "sigmaos/maze"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
	"sigmaos/rand"
	"sigmaos/test"
	"strconv"
	"testing"
)

// XXX BUILD ERROR XXX package sigmaos/mazes is not in GOROOT (/go/custom/src/sigmaos/mazes)

// I'm not going to cache or database this, even though it's extremely network intensive,
// because I want a new maze every time the request is called.

type TstateMaze struct {
	*test.Tstate
	jobname string
	pid     proc.Tpid
}

func makeTstateMaze(t *testing.T, job string) (*TstateMaze, error) {
	// Init
	tse := TstateMaze{}
	tse.jobname = job
	tse.Tstate = test.MakeTstateAll(t)

	// Setup working namespace
	if err := tse.MkDir(maze.DIR_MAZE, 0777); err != nil {
		db.DFatalf("|%v| Error setting up the working namespace for Maze when creating %v directory: %v", job, maze.DIR_MAZE, err)
		return nil, err
	}
	jobDir := path.Join(maze.DIR_MAZE, tse.jobname)
	if err := tse.MkDir(jobDir, 0777); err != nil {
		db.DFatalf("|%v| Error setting up the working namespace for Maze when creating %v directory: %v", job, jobDir, err)
		return nil, err
	}

	// Setup main proc
	// XXX I have no idea why we use test.Overlays as the public bool
	p := proc.MakeProc("maze-main", []string{strconv.FormatBool(test.Overlays)})
	// XXX Should this be more because it's kind of resource intensive?
	p.SetNcore(proc.Tcore(1))
	if err := tse.Spawn(p); err != nil {
		db.DFatalf("|%v| Error spawning proc %v: %v", job, p, err)
		return nil, err
	}
	if err := tse.WaitStart(p.GetPid()); err != nil {
		db.DFatalf("|%v| Error waiting for proc %v to start: %v", job, p, err)
		return nil, err
	}

	// XXX why do we need this?
	tse.pid = p.GetPid()
	return &tse, nil
}

// XXX Review why this is necessary
func (tsm *TstateMaze) Stop() error {
	if err := tsm.Evict(tsm.pid); err != nil {
		return err
	}
	if _, err := tsm.WaitExit(tsm.pid); err != nil {
		return err
	}
	return tsm.Shutdown()
}

func TestMaze(t *testing.T) {
	// Start server
	tsm, err := makeTstateMaze(t, rand.String(8))
	assert.Nil(t, err, "makeTstateMaze failed: %v", err)

	// Create an RPC client
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{tsm.FsLib}, maze.NAMED_MAZE_SERVER)
	assert.Nil(t, err, "ProtDevClnt creation failed: %v", err)

	// Request maze from server
	arg := maze.MazeRequest{}
	res := maze.MazeResponse{}
	err = pdc.RPC("Maze.SolveMaze", &arg, &res)
	assert.Nil(t, err, "Maze RPC call failed with arg: %v and err: %v", arg, err)
	db.DPrintf(maze.DEBUG_MAZE, "Maze Output: %v", res.GetMaze())

	// Stop server
	// assert.Nil(t, tsm.Stop())
}

package mazesrv_test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"os/exec"
	"path"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/maze"
	"sigmaos/mazesrv"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
	"sigmaos/rand"
	"sigmaos/test"
	"strconv"
	"testing"
)

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
	var err error
	if err = mazesrv.InitMazeNamespace(tse.Tstate.SigmaClnt.FsLib, job); err != nil {
		db.DFatalf("|%v| Error initializing Maze namespace: %v", job, err)
		return nil, err
	}
	//if err = maze.InitBFSNamespace(tse.Tstate.SigmaClnt.FsLib, job); err != nil {
	//	db.DFatalf("|%v| Error initializing BFS namespace: %v", job, err)
	//	return nil, err
	//}
	// Setup main proc
	db.DPrintf(mazesrv.DEBUG_MAZE, "|%v| Spawning Proc", job)
	p := proc.MakeProc("maze-main", []string{strconv.FormatBool(test.Overlays)})
	p.SetNcore(proc.Tcore(1))
	if err = tse.Spawn(p); err != nil {
		db.DFatalf("|%v| Error spawning proc %v: %v", job, p, err)
		return nil, err
	}
	if err = tse.WaitStart(p.GetPid()); err != nil {
		db.DFatalf("|%v| Error waiting for proc %v to start: %v", job, p, err)
		return nil, err
	}
	db.DPrintf(mazesrv.DEBUG_MAZE, "|%v| Done with Initialization", job)
	tse.pid = p.GetPid()
	return &tse, nil
}

func (tsm *TstateMaze) Stop() error {
	if err := tsm.Evict(tsm.pid); err != nil {
		return err
	}
	if _, err := tsm.WaitExit(tsm.pid); err != nil {
		return err
	}
	return tsm.Shutdown()
}

func viewHTML(t *testing.T, html []byte) {
	fn := "maze.html"
	fo, err := os.Create(fn)
	assert.Nil(t, err, "Failed to create file %v", fn)
	defer func(fo *os.File) {
		err := fo.Close()
		assert.Nil(t, err, "Failed to close file %v", fn)
	}(fo)

	_, err = fo.Write(html)
	assert.Nil(t, err, "Failed to write to file %v", fn)
	err = exec.Command("xdg-open", fmt.Sprintf("%v", fn)).Run()
	assert.Nil(t, err, "Failed to run command 'xdg-open %v'", fn)
}

func TestMaze(t *testing.T) {
	// Start server
	tsm, err := makeTstateMaze(t, rand.String(8))
	assert.Nil(t, err, "makeTstateMaze failed: %v", err)

	// Create an RPC client
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{tsm.FsLib}, path.Join(mazesrv.NAMED_MAZE_SERVER, "~any"))
	assert.Nil(t, err, "ProtDevClnt creation failed: %v", err)

	// Request maze from server
	arg := mazesrv.MazeRequest{
		Height:      100,
		Width:       100,
		GenerateAlg: maze.GEN_DFS,
		SolveAlg:    maze.SOLVE_BFS_MULTI,
		Repeats:     50,
	}
	res := mazesrv.MazeResponse{}
	err = pdc.RPC("Maze.GetMaze", &arg, &res)
	assert.Nil(t, err, "Maze RPC call failed with arg: %v and err: %v", arg, err)
	// Both ways to view output interrupt testing, so are commented out
	// db.DPrintf(mazesrv.DEBUG_MAZE, "Maze Output: %v", res.GetWebpage())
	// viewHTML(t, []byte(res.GetWebpage()))
	// Stop server
	assert.Nil(t, tsm.Stop())
}

//
//	func TestWebsite(t *testing.T) {
//		mux := http.NewServeMux()
//		mux.HandleFunc("/", mazesrv.MakeMazeResponse)
//
//		port := "3000"
//		http.ListenAndServe(":"+port, mux)
//	}
//

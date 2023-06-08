package mazesrv

import (
	"html/template"
	"io"
	"math"
	"net/http"
	db "sigmaos/debug"
	"sigmaos/maze"
	"strconv"
	"time"
)

var tpl = template.Must(template.New("MazeHTML").Parse(MAZEHTML))

type MazeInputs struct {
	width      int
	height     int
	tickSpeed  int
	repeats    int
	density    int
	solveAlg   string
	genAlg     string
	startIndex int
}

// fix corrects to default if a value out of a reasonable range.
// To create a default maze, set all integer values to -1 and call this.
// Does not check algorithms; they are checked by the maze exporter.
func (in *MazeInputs) fix() {
	// These numbers are arbitrary, based on current algorithm
	// efficiency and how long I'm willing to wait.
	if in.width < 3 || in.width > 1000 {
		in.width = 200
	}
	if in.height < 3 || in.height > 1000 {
		in.height = 112
	}
	if in.tickSpeed <= 0 {
		in.tickSpeed = 1
	}
	if in.repeats <= 0 {
		in.repeats = 100
	}
	if in.density <= 0 {
		in.density = 15
	}
	// XXX Make sure my math is correct for both bounds
	if in.startIndex < 0 || in.startIndex > ((in.width*in.height)-1) {
		in.startIndex = 0
	}
	if in.solveAlg != maze.SOLVE_BFS_MULTI && in.solveAlg != maze.SOLVE_BFS_SINGLE && in.solveAlg != maze.SOLVE_DFS_MULTI {
		in.solveAlg = maze.SOLVE_BFS_MULTI
	}
	if in.genAlg != maze.GEN_DFS && in.genAlg != maze.GEN_RAND && in.genAlg != maze.GEN_NONE {
		in.genAlg = maze.GEN_DFS
	}
}

func (in *MazeInputs) getFormData() string {
	// I know this is gross, sorry.
	return "[\"" + in.genAlg + "\", \"" + in.solveAlg + "\", \"" + strconv.Itoa(in.width) + "\", \"" + strconv.Itoa(in.height) + "\", \"" + strconv.Itoa(in.tickSpeed) + "\", \"" + strconv.Itoa(in.repeats) + "\", \"" + strconv.Itoa(in.density) + "\"]"
}

func makeSolveMaze(in *MazeInputs, wr io.Writer) error {
	in.fix()

	timeStart := time.Now()
	tplData, err := fillTemplateData(in)
	timeEnd := time.Now()

	if err != nil {
		return err
	}
	printTime(timeStart, timeEnd, "Served maze")
	return tpl.Execute(wr, tplData)
}

// MakeMazeResponse converts a http request into a http response
func MakeMazeResponse(wr http.ResponseWriter, rd *http.Request) {
	// Parse values from GET request, if they exist
	w, err := strconv.Atoi(rd.URL.Query().Get("width"))
	if err != nil {
		w = -1
	}
	h, err := strconv.Atoi(rd.URL.Query().Get("height"))
	if err != nil {
		h = -1
	}
	ts, err := strconv.Atoi(rd.URL.Query().Get("tickSpeed"))
	if err != nil {
		ts = -1
	}
	r, err := strconv.Atoi(rd.URL.Query().Get("repeats"))
	if err != nil {
		r = -1
	}
	d, err := strconv.Atoi(rd.URL.Query().Get("density"))
	if err != nil {
		d = -1
	}
	sa := rd.URL.Query().Get("solveAlgorithm")
	ga := rd.URL.Query().Get("generateAlgorithm")

	// Calculate and display maze results
	// XXX TODO Sometimes there are visual glitches in the maze display
	in := MazeInputs{
		width:      w,
		height:     h,
		tickSpeed:  ts,
		repeats:    r,
		density:    d,
		solveAlg:   sa,
		genAlg:     ga,
		startIndex: 0,
	}
	err = makeSolveMaze(&in, wr)
	if err != nil {
		db.DPrintf(DEBUG_MAZE, "Maze error: %v\n", err)
	}
}

func printTime(timeStart time.Time, timeEnd time.Time, msg string) {
	// Manually calculate times from nanoseconds to have control over rounding
	timeEndNs := timeEnd.UnixNano() - timeStart.UnixNano()
	timeEndUs := float64(timeEndNs) / 1000.0
	timeEndMs := timeEndUs / 1000.0
	db.DPrintf(DEBUG_MAZE, "%v in %.0f ms and %.0f us\n", msg, timeEndMs, timeEndUs-(math.Floor(timeEndMs)*1000.0))
}

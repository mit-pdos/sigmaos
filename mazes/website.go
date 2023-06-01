package main

import (
	"fmt"
	"html/template"
	"math"
	"net/http"
	"strconv"
	"time"
)

var tpl = template.Must(template.ParseFiles("templates/index.html"))

// makeMaze converts a http request into a http response
func makeMazeResponse(w http.ResponseWriter, r *http.Request) {
	timeStart := time.Now()
	// generationAlgorithm defines the maze generation algorithm
	// 1 = random
	// 2 = DFS

	// Default configuration variables
	width := 200
	height := 112
	tickSpeed := 1
	repeats := 100
	density := 15
	solve := ""
	generate := ""

	// Parse values from GET request, if they exist
	inputWidth, err := strconv.Atoi(r.URL.Query().Get("width"))
	if err == nil && inputWidth > 2 {
		width = inputWidth
	}
	inputHeight, err := strconv.Atoi(r.URL.Query().Get("height"))
	if err == nil && inputHeight > 2 {
		height = inputHeight
	}
	inputTickSpeed, err := strconv.Atoi(r.URL.Query().Get("tickSpeed"))
	if err == nil && inputTickSpeed > 0 {
		tickSpeed = inputTickSpeed
	}
	inputRepeats, err := strconv.Atoi(r.URL.Query().Get("repeats"))
	if err == nil && inputRepeats > 0 {
		repeats = inputRepeats
	}
	inputDensity, err := strconv.Atoi(r.URL.Query().Get("density"))
	if err == nil && inputDensity > 0 {
		density = inputDensity
	}
	inputSolve := r.URL.Query().Get("solveAlgorithm")
	switch inputSolve {
	case "dfs":
		solve = "dfs"
	case "bfsmulti":
		solve = "bfsmulti"
	case "bfs":
		solve = "bfs"
	}
	inputGenerate := r.URL.Query().Get("generateAlgorithm")
	switch inputGenerate {
	case "random":
		generate = "random"
	case "dfs":
		generate = "dfs"
	}

	// I know this is gross, sorry.
	formData := "[\"" + generate + "\", \"" + solve + "\", \"" + strconv.Itoa(width) + "\", \"" + strconv.Itoa(height) + "\", \"" + strconv.Itoa(tickSpeed) + "\", \"" + strconv.Itoa(repeats) + "\", \"" + strconv.Itoa(density) + "\"]"

	// Init maze with a given algorithm
	maze := InitMaze(height, width)
	maze.SetSquare(height-1, width-1, 3)
	switch generate {
	case "random":
		RandomizeMaze(maze, density)
	case "dfs":
		CreateDFSMaze(maze)
	default:
		maze.SetAllWalls(false)
	}

	// Solve maze with a given algorithm
	var tplData *TemplateData
	switch solve {
	case "bfs":
		tplData = fillTemplateBFS(maze, tickSpeed, repeats, formData)
	case "dfs":
		tplData = fillTemplateDFS(maze, tickSpeed, repeats, formData)
	case "bfsmulti":
		tplData = fillTemplateBFSMultithreaded(maze, tickSpeed, repeats, formData)
	default:
		tplData = fillTemplateData(maze, "[]", "[]", tickSpeed, repeats, formData)
	}

	// TODO Sometimes there are visual glitches in the maze display
	err = tpl.Execute(w, tplData)
	if err != nil {
		fmt.Println(err)
		return
	}

	timeEnd := time.Now()
	// Manually calculate times with ns to have control over rounding
	timeEndNs := timeEnd.UnixNano() - timeStart.UnixNano()
	timeEndUs := float64(timeEndNs) / 1000.0
	timeEndMs := timeEndUs / 1000.0
	fmt.Printf("Served! in %.0f ms and %.0f us\n", timeEndMs, timeEndUs-(math.Floor(timeEndMs)*1000.0))
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", makeMazeResponse)

	port := "3000"
	http.ListenAndServe(":"+port, mux)
}

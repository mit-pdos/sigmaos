package main

import (
	"html/template"
	"strconv"
)

type TemplateData struct {
	// Template data structs must have exported names so the template Executer can read them.

	// MStyles contains the CSS Style text for every node
	MStyles [][]template.CSS
	// MPath contains the first-executed searching path
	MPath template.JS
	// MBestPath contains the second-executed solution path
	MBestPath template.JS
	// TickSpeed determines how fast each node updates when drawing paths
	TickSpeed template.JS
	// PathRepeats determines the number of nodes updated for each tick when drawing paths
	PathRepeats template.JS
	// FormData allows for user inputted form data to reappear on the webpage
	FormData template.JS
}

func toStyle(node mazeNode) template.CSS {
	out := ""

	// assumes the maze is empty to start, except for solution.
	if node.val == 3 {
		out += "c-goal "
	}

	// These define borders.
	if node.up {
		out += "b-t "
	}
	if node.down {
		out += "b-b "
	}
	if node.right {
		out += "b-r "
	}
	if node.left {
		out += "b-l "
	}

	return template.CSS(out)
}

func mazeSliceToStyle(mazeVals [][]mazeNode) [][]template.CSS {
	mazeStyles := make([][]template.CSS, len(mazeVals))
	for row := range mazeVals {
		newRow := make([]template.CSS, len(mazeVals[0]))
		for col := range mazeVals[row] {
			newRow[col] = toStyle(mazeVals[row][col])
		}
		mazeStyles[row] = newRow
	}
	return mazeStyles
}

func pathToJs(m *maze, path *[]int) template.JS {
	if len(*path) == 0 {
		return template.JS("[]")
	}

	out := "["

	for i := 0; i < len(*path); i++ {
		row, col := GetMazeCoords(m, (*path)[i])
		out += "[" + strconv.Itoa(row) + ", " + strconv.Itoa(col) + "], "
	}

	// Cut off trailing comma
	out = out[:len(out)-2]
	out += "]"

	return template.JS(out)
}

func pathsToJs(m *maze, paths *[][]int) template.JS {
	out := template.JS("[")
	for _, path := range *paths {
		out += pathToJs(m, &path)
		out += ", "
	}
	// Cut off trailing comma
	out = out[:len(out)-2]
	out += template.JS("]")
	return out
}

func fillTemplateBFS(m *maze, tickSpeed int, repeats int, formData string) *TemplateData {
	// Run BFS to find a solution
	bfsOk, bfsPath, bfsSolution := BFS(&m.g, 3, 0)

	var mazePath template.JS
	// Even if it fails, show what it got before it failed
	mazePath = "[" + pathToJs(m, bfsPath) + "]"

	var bestPath template.JS
	if bfsOk {
		bestPath = pathToJs(m, bfsSolution)
	} else {
		print("No Valid BFS\n")
		bestPath = template.JS("[]")
	}

	return fillTemplateData(m, mazePath, bestPath, tickSpeed, repeats, formData)
}

func fillTemplateBFSMultithreaded(m *maze, tickSpeed int, repeats int, formData string) *TemplateData {
	// Run BFS to find a solution
	bfsOk, bfsPath, bfsSolution := BFSMultithreaded(&m.g, 3, 0, 4)

	var mazePath template.JS
	// Even if it fails, show what it got before it failed
	mazePath = pathsToJs(m, bfsPath)

	var bestPath template.JS
	if bfsOk {
		bestPath = pathToJs(m, bfsSolution)
	} else {
		print("No Valid BFS\n")
		bestPath = template.JS("[]")
	}

	return fillTemplateData(m, mazePath, bestPath, tickSpeed, repeats, formData)
}

func fillTemplateDFS(m *maze, tickSpeed int, repeats int, formData string) *TemplateData {
	// Run DFS to find a solution
	dfsOk, dfsPath := DFS(&m.g, 3, 0)
	var bestPath template.JS
	if dfsOk {
		bestPath = pathToJs(m, dfsPath)
	} else {
		print("No Valid DFS\n")
		bestPath = template.JS("[]")
	}

	// Run Multithreaded DFS to find a solution
	startI := GetSeekerLocations(m, 4)
	multithreadedOk, paths := DFSMultithreaded(&m.g, 3, startI)
	var mazePath template.JS
	// Even if it fails, show what it got before it failed
	mazePath = pathsToJs(m, paths)
	if multithreadedOk == -1 {
		print("No Valid Multi-threaded DFS\n")
	}

	return fillTemplateData(m, mazePath, bestPath, tickSpeed, repeats, formData)
}

// fillTemplateData executes the search algorithms and processes their results.
func fillTemplateData(m *maze, mazePath template.JS, bestPath template.JS, tickSpeed int, repeats int, formData string) *TemplateData {
	mazeValues := mazeToSlice(m)
	mazeStyles := mazeSliceToStyle(mazeValues)

	// Convert data into the types that a template can take
	tplData := TemplateData{
		MStyles:     mazeStyles,
		MPath:       mazePath,
		MBestPath:   bestPath,
		TickSpeed:   template.JS(strconv.Itoa(tickSpeed)),
		PathRepeats: template.JS(strconv.Itoa(repeats)),
		FormData:    template.JS(formData),
	}
	return &tplData
}

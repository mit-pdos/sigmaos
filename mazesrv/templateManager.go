package mazesrv

import (
	"html/template"
	"sigmaos/maze"
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

func toStyle(node maze.MNode) template.CSS {
	out := ""
	// assumes the maze is empty to start, except for solution.
	if node.Val == 3 {
		out += "c-goal "
	}
	if node.Up {
		out += "b-t "
	}
	if node.Down {
		out += "b-b "
	}
	if node.Right {
		out += "b-r "
	}
	if node.Left {
		out += "b-l "
	}
	return template.CSS(out)
}

func mazeSliceToStyle(mazeVals *[][]maze.MNode) [][]template.CSS {
	mazeStyles := make([][]template.CSS, len(*mazeVals))
	for row := range *mazeVals {
		newRow := make([]template.CSS, len((*mazeVals)[0]))
		for col := range (*mazeVals)[row] {
			newRow[col] = toStyle((*mazeVals)[row][col])
		}
		mazeStyles[row] = newRow
	}
	return mazeStyles
}

func pathToJs(mazeWidth int, path *[]int) template.JS {
	if len(*path) == 0 {
		return "[]"
	}

	out := "["
	for i := 0; i < len(*path); i++ {
		row, col := maze.GetSquareCoords((*path)[i], mazeWidth)
		out += "[" + strconv.Itoa(row) + ", " + strconv.Itoa(col) + "], "
	}

	// Cut off trailing comma
	out = out[:len(out)-2]
	out += "]"
	return template.JS(out)
}

func pathsToJs(mazeWidth int, paths *[][]int) template.JS {
	out := template.JS("[")
	for _, path := range *paths {
		out += pathToJs(mazeWidth, &path)
		out += ", "
	}
	// Cut off trailing comma
	out = out[:len(out)-2]
	out += template.JS("]")
	return out
}

func fillTemplateData(in *MazeInputs) (*TemplateData, error) {
	m, p, b, err := maze.MakeSolveMaze(in.width, in.height, in.density, in.genAlg, in.solveAlg, in.startIndex)
	if err != nil {
		return nil, err
	}

	tplData := TemplateData{
		MStyles:     mazeSliceToStyle(m),
		MPath:       pathsToJs(in.width, p),
		MBestPath:   pathToJs(in.width, b),
		TickSpeed:   template.JS(strconv.Itoa(in.tickSpeed)),
		PathRepeats: template.JS(strconv.Itoa(in.repeats)),
		FormData:    template.JS(in.getFormData()),
	}
	return &tplData, nil
}

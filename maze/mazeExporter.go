package maze

import "errors"

type mazeNode struct {
	val int
	// If true, there is a wall in that direction.
	// If false, there is no wall in that direction (which is an edge in the underlying graph).
	up    bool
	down  bool
	right bool
	left  bool
}

// Prefixes
const (
	GEN   = "GEN_"
	SOLVE = "SOLVE_"
)

// Suffixes
const (
	MULTI  = "_MULTI"
	SINGLE = "_SINGLE"
)

// Full selectors
const (
	GEN_DFS  = GEN + "DFS"
	GEN_RAND = GEN + "RAND"
	GEN_NONE = GEN + "NONE"

	SOLVE_DFS_MULTI  = SOLVE + "DFS" + MULTI
	SOLVE_BFS_SINGLE = SOLVE + "BFS" + SINGLE
	SOLVE_BFS_MULTI  = SOLVE + "BFS" + MULTI
)

func mkErr(message string) error {
	return errors.New("Maze: " + message)
}

func makeMazeNode(m *maze, row int, col int) mazeNode {
	var newMazeNode mazeNode
	newMazeNode.val = m.g.nodes[GetMazeIndex(m, row, col)].val
	// An edge is the absence of a wall
	// If it has no edge, it has a wall
	if row == 0 || !m.g.HasEdge(GetMazeIndex(m, row, col), GetMazeIndex(m, row-1, col)) {
		newMazeNode.up = true
	}
	if row == m.height-1 || !m.g.HasEdge(GetMazeIndex(m, row, col), GetMazeIndex(m, row+1, col)) {
		newMazeNode.down = true
	}
	if col == m.width-1 || !m.g.HasEdge(GetMazeIndex(m, row, col), GetMazeIndex(m, row, col+1)) {
		newMazeNode.right = true
	}
	if col == 0 || !m.g.HasEdge(GetMazeIndex(m, row, col), GetMazeIndex(m, row, col-1)) {
		newMazeNode.left = true
	}

	return newMazeNode
}

// mazeToSlice outputs a 2d slice that contains the values of every node as well as boolean values representing the existence of a wall up, down, right, and left.
func mazeToSlice(m *maze) [][]mazeNode {
	nodes := make([][]mazeNode, m.height)
	for row := 0; row < m.height; row++ {
		newRow := make([]mazeNode, m.width)
		for col := 0; col < m.width; col++ {
			newRow[col] = makeMazeNode(m, row, col)
		}
		nodes[row] = newRow
	}
	return nodes
}

func makeMaze(width int, height int, density int, generateAlg string) (*maze, error) {
	// Init maze with a given algorithm
	maze := InitMaze(height, width)
	maze.SetSquare(height-1, width-1, 3)
	switch generateAlg {
	case GEN_RAND:
		RandomizeMaze(maze, density)
	case GEN_DFS:
		CreateDFSMaze(maze)
	case GEN_NONE:
		maze.SetAllWalls(false)
	default:
		return nil, mkErr("invalid generation algorithm")
	}
	return maze, nil
}

func solveMaze(m *maze, solveAlg string) (*[][]int, *[]int, error) {
	if m == nil {
		return nil, nil, mkErr("invalid maze")
	}
	var ok bool
	var okDFS int
	var searchPaths *[][]int
	var best *[]int
	switch solveAlg {
	case SOLVE_DFS_MULTI:
		ok, best = DFS(&m.g, 3, 0)
		if !ok {
			return nil, nil, mkErr("DFS singlethreaded failed")
		}
		okDFS, searchPaths = DFSMultithreaded(&m.g, 3, GetSeekerLocations(m, 4))
		if okDFS == -1 {
			return nil, nil, mkErr("DFS multithreaded failed")
		}
	case SOLVE_BFS_MULTI:
		ok, searchPaths, best = BFSMultithreaded(&m.g, 3, 0, 4)
		if !ok {
			return nil, nil, mkErr("BFS multithreaded failed")
		}
	case SOLVE_BFS_SINGLE:
		ok, searchPaths, best = BFS(&m.g, 3, 0)
		if !ok {
			return nil, nil, mkErr("BFS singlethreaded failed")
		}
	default:
		return nil, nil, mkErr("invalid solving algorithm")
	}
	return searchPaths, best, nil
}

package maze

import (
	"math/rand"
)

const (
	NODE_EMPTY = iota
	NODE_PATHS
	NODE_BEST
	NODE_GOAL
)

type maze struct {
	g      graph
	height int
	width  int
}

func initMaze(height int, width int) *maze {
	// Cannot be smaller than 2x2
	if height < 2 || width < 2 {
		return nil
	}

	totalNodes := height * width
	var m maze
	g := &(m.g)
	for i := 0; i < totalNodes; i++ {
		g.addNode(NODE_EMPTY)
	}

	m.height = height
	m.width = width
	return &m
}

// setAllWalls makes all the walls the same value, depending on remove
// If exists is true, all walls are added (no edges)
// If exists is false, all walls are removed (edges between every node and its neighbors)
func (m *maze) setAllWalls(exists bool) {
	// Height is the number of rows, width is the number of columns
	// Initialize with no walls, so every node is connected to the node to its Top, Bottom, Right, and Left
	// Nodes fill left to right, then top to bottom
	// Edges are added on  the bottom and right side, so the bottom row and right column are ignored, which is seen in the height-1 and width-1
	for row := 0; row < m.height-1; row++ {
		for col := 0; col < m.width-1; col++ {
			//edge to below
			m.setWall(row, col, row+1, col, !exists)
			//edge to the right
			m.setWall(row, col, row, col+1, !exists)
		}
		// Add just below for the right column
		m.setWall(row, m.width-1, row+1, m.width-1, !exists)
	}
	// Add just to the right for the bottom row, and nothing for the bottom right node, which is seen in the width-1
	for col := 0; col < m.width-1; col++ {
		m.setWall(m.height-1, col, m.height-1, col+1, !exists)
	}
}

func getMazeIndex(m *maze, row int, col int) int {
	return row*(m.width) + col
}

// getMazeCoords returns (row, col) from index
func getMazeCoords(m *maze, index int) (int, int) {
	return GetSquareCoords(index, m.width)
}

func GetSquareCoords(index int, width int) (int, int) {
	return index / width, index % width
}

func (m *maze) setSquare(row int, col int, val int) {
	m.g.setNode(getMazeIndex(m, row, col), val)
}

func (m *maze) setWall(row1 int, col1 int, row2 int, col2 int, remove bool) {
	// For direction: 1 = up, 2 = down, 3 = right, 4 = left
	// For removing: true = remove, false = add

	//TODO error checking to make sure the nodes are next to each other
	index1 := getMazeIndex(m, row1, col1)
	index2 := getMazeIndex(m, row2, col2)

	// Adding an edge removes a wall
	// Removing an edge adds a wall
	// This is because graph algorithms can only travel over edges, so they must be gaps in the wall
	if remove {
		m.g.addEdge(index1, index2)
	} else {
		m.g.removeEdge(index1, index2)
	}
}

// randomizeMaze randomizes every wall in the maze
// Increased density increases the number of walls; density=20 will have half the walls filled.
func randomizeMaze(m *maze, density int) {
	for row := 0; row < m.height-1; row++ {
		for col := 0; col < m.width-1; col++ {
			// Randomize edge below
			m.setWall(row, col, row+1, col, rand.Intn(density) < 10)
			// Randomize edge to the right
			m.setWall(row, col, row, col+1, rand.Intn(density) < 10)
		}
		// Randomize just below for the right column
		m.setWall(row, m.width-1, row+1, m.width-1, rand.Intn(density) < 10)
	}
	// Randomize just to the right for the bottom row, and nothing for the bottom right node
	for col := 0; col < m.width-1; col++ {
		m.setWall(m.height-1, col, m.height-1, col+1, rand.Intn(density) < 10)
	}
}

func possibleNeighbors(m *maze, row int, col int) [][]int {
	neighbors := make([][]int, 0)
	if row > 0 {
		neighbors = append(neighbors, []int{row - 1, col})
	}
	if row < m.height-1 {
		neighbors = append(neighbors, []int{row + 1, col})
	}
	if col > 0 {
		neighbors = append(neighbors, []int{row, col - 1})
	}
	if col < m.width-1 {
		neighbors = append(neighbors, []int{row, col + 1})
	}
	return neighbors
}

// createDFSMaze generates a new maze using backtracking DFS.
// First, it fills the maze with walls.
// Then it runs DFS with no end condition, stopping once every node has been visited once.
// Every time DFS moves between two nodes, it removes the wall in its way.
func createDFSMaze(m *maze) {
	// Wipe the maze, filling with all walls
	m.setAllWalls(true)

	visited := make([][]bool, m.height, m.height)
	for i := range visited {
		visited[i] = make([]bool, m.width, m.width)
	}

	createDFSMazeRecursive(m, 0, 0, &visited)
}

func createDFSMazeRecursive(m *maze, row int, col int, visited *[][]bool) {
	(*visited)[row][col] = true

	// Nothing has neighbors because everything is wiped
	neighbors := possibleNeighbors(m, row, col)
	for len(neighbors) > 0 {
		index := rand.Intn(len(neighbors))
		row2 := neighbors[index][0]
		col2 := neighbors[index][1]

		if !(*visited)[row2][col2] {
			m.setWall(row, col, row2, col2, true)
			createDFSMazeRecursive(m, row2, col2, visited)
		}
		neighbors = append(neighbors[:index], neighbors[index+1:]...)
	}
}

func (m *maze) fillPath(path []int, val int) {
	// Reverse path to draw from starting location
	// Skip the first item which would overwrite the solution
	for i := len(path) - 1; i >= 1; i-- {
		row, col := getMazeCoords(m, path[i])
		m.setSquare(row, col, val)
	}
}

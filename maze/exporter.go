package maze

import (
	"errors"
	"fmt"
)

type MNode struct {
	Val int
	// If true, there is a wall in that direction.
	// If false, there is no wall in that direction (which is an edge in the underlying graph).
	Up    bool
	Down  bool
	Right bool
	Left  bool
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

func makeMNode(m *maze, row int, col int) MNode {
	var newMNode MNode
	newMNode.Val = m.g.nodes[getMazeIndex(m, row, col)].val
	// An edge is the absence of a wall
	// If it has no edge, it has a wall
	if row == 0 || !m.g.hasEdge(getMazeIndex(m, row, col), getMazeIndex(m, row-1, col)) {
		newMNode.Up = true
	}
	if row == m.height-1 || !m.g.hasEdge(getMazeIndex(m, row, col), getMazeIndex(m, row+1, col)) {
		newMNode.Down = true
	}
	if col == m.width-1 || !m.g.hasEdge(getMazeIndex(m, row, col), getMazeIndex(m, row, col+1)) {
		newMNode.Right = true
	}
	if col == 0 || !m.g.hasEdge(getMazeIndex(m, row, col), getMazeIndex(m, row, col-1)) {
		newMNode.Left = true
	}

	return newMNode
}

// mazeToSlice outputs a 2d slice that contains the values of every node as well as boolean values representing the existence of a wall up, down, right, and left.
func mazeToSlice(m *maze) *[][]MNode {
	nodes := make([][]MNode, m.height)
	for row := 0; row < m.height; row++ {
		newRow := make([]MNode, m.width)
		for col := 0; col < m.width; col++ {
			newRow[col] = makeMNode(m, row, col)
		}
		nodes[row] = newRow
	}
	return &nodes
}

//	UNUSED IMPORT/EXPORT FUNCTIONS
//
//	func reverseMNode(m *maze, n MNode, row int, col int) {
//		m.setSquare(row, col, n.Val)
//		// Inverting the values because true for MNode means there is a wall,
//		// and false for setWall means add a wall
//		if row != 0 {
//			m.setWall(row, col, row-1, col, !n.Up)
//		}
//		if row != m.height-1 {
//			m.setWall(row, col, row+1, col, !n.Down)
//		}
//		if col != m.width-1 {
//			m.setWall(row, col, row, col+1, !n.Right)
//		}
//		if col != 0 {
//			m.setWall(row, col, row, col-1, !n.Left)
//		}
//	}
//
//	func sliceToMaze(nodes *[][]MNode) *maze {
//		m := initMaze(len(*nodes), len((*nodes)[0]))
//		for row := 0; row < m.height; row++ {
//			for col := 0; col < m.width; col++ {
//				reverseMNode(m, (*nodes)[row][col], row, col)
//			}
//		}
//		return m
//	}
//
//
//	func graphToSlice(g *graph) *[][]int {
//		out := make([][]int, len(g.nodes))
//		for _, node := range g.nodes {
//			n := make([]int, len(node.neighbors))
//			for _, neighbor := range node.neighbors {
//				n = append(n, neighbor.n.index)
//			}
//			out = append(out, n)
//		}
//		return &out
//	}
//
//	func sliceToGraph(s *[][]int) *graph {
//		g := graph{
//			nodes: make([]*node, len(*s)),
//		}
//		for range *s {
//			g.addNode(1)
//		}
//		for i, node := range *s {
//			for _, adj := range node {
//				g.addEdge(i, adj)
//			}
//		}
//		return &g
//	}
//
//
//	func marshal(g *graph) (*[]byte, error) {
//		s := graphToSlice(g)
//		out, err := json.Marshal(s)
//		if err != nil {
//			return nil, err
//		}
//		return &out, nil
//	}
//
//	func unmarshal(s []byte, g *graph) error {
//		var gr *[][]int
//		if err := json.Unmarshal(s, gr); err != nil {
//			return err
//		}
//		*g = *sliceToGraph(gr)
//		return nil
//	}
//

func makeMaze(width int, height int, density int, generateAlg string) (*maze, error) {
	// Init maze with a given algorithm
	maze := initMaze(height, width)
	maze.setSquare(height-1, width-1, NODE_GOAL)
	switch generateAlg {
	case GEN_RAND:
		randomizeMaze(maze, density)
	case GEN_DFS:
		createDFSMaze(maze)
	case GEN_NONE:
		maze.setAllWalls(false)
	default:
		return nil, mkErr("invalid generation algorithm")
	}
	return maze, nil
}

func solveMaze(m *maze, solveAlg string, startIndex int) (*[][]int, *[]int, error) {
	if m == nil {
		return nil, nil, mkErr("invalid maze")
	}
	var ok bool
	var err error
	var searchPaths *[][]int
	var best *[]int
	switch solveAlg {
	case SOLVE_DFS_MULTI:
		ok, best = dfs(&m.g, 3, startIndex)
		if !ok {
			return nil, nil, mkErr("DFS singlethreaded failed")
		}
		ok, searchPaths = dfsMultithreaded(&m.g, 3, getSeekerLocations(m, 4))
		if !ok {
			return nil, nil, mkErr("DFS multithreaded failed")
		}
	case SOLVE_BFS_MULTI:
		searchPaths, best, err = bfsMultithreaded(&m.g, 3, startIndex, 4)
		if err != nil {
			return nil, nil, mkErr(fmt.Sprintf("BFS multithreaded failed: %v", err))
		}
	case SOLVE_BFS_SINGLE:
		ok, searchPaths, best = bfs(&m.g, 3, startIndex)
		if !ok {
			return nil, nil, mkErr("BFS singlethreaded failed")
		}
	default:
		return nil, nil, mkErr("invalid solving algorithm")
	}
	return searchPaths, best, nil
}

// MakeSolveMaze returns (maze as slice, all paths, best path, error)
func MakeSolveMaze(width int, height int, density int, generateAlg string, solveAlg string, startIndex int) (*[][]MNode, *[][]int, *[]int, error) {
	m, err := makeMaze(width, height, density, generateAlg)
	if err != nil {
		return nil, nil, nil, err
	}
	p, b, err := solveMaze(m, solveAlg, startIndex)
	if err != nil {
		return nil, nil, nil, err
	}
	return mazeToSlice(m), p, b, nil
}

//
//	func MakeMaze(width int, height int, density int, generateAlg string) (*[]byte, error) {
//		m, err := makeMaze(width, height, density, generateAlg)
//		if err != nil {
//			return nil, err
//		}
//		marshaled, err := marshal(&m.g)
//		if err != nil {
//			return nil, err
//		}
//		return marshaled, nil
//	}
//
//	func SolveMaze(maze *[]byte, solveAlg string, startIndex int) (*[][]int, *[]int, error) {
//				var g *graph
//		if err := unmarshal(*maze, g); err != nil {
//			return nil, nil, err
//		}
//		p, b, err := solveMaze(g, solveAlg, startIndex)
//		if err != nil {
//			return nil, nil, err
//		}
//		return p, b, nil
//	}
//

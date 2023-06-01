package main

type mazeNode struct {
	val int
	// If true, there is a wall in that direction.
	// If false, there is no wall in that direction (which is an edge in the underlying graph).
	up    bool
	down  bool
	right bool
	left  bool
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

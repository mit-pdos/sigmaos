package maze

import "sync"

// getSeekerLocations is a helper function to get DFS seekers' initial locations
func getSeekerLocations(m *maze, numSeekers int) []int {
	// Math to place evenly spaced seekers in the middle of the maze
	spacerForIndex := m.width / numSeekers
	rowForIndex := m.height*m.width/2 - spacerForIndex/2
	starts := make([]int, numSeekers, numSeekers)

	for i := 0; i < numSeekers; i++ {
		starts[i] = rowForIndex + spacerForIndex*i
	}
	return starts
}

// dfs finds the first node with a given value and returns:
// - a boolean which is true if the value is accessible
// - a slice of indexes with the order of nodes to get there, starting with the
// node of the desired value and ending with the starting node
func dfs(g *graph, val int, startIndex int) (exists bool, path *[]int) {
	pathOut := make([]int, 0)
	visited := make([]bool, len(g.nodes), len(g.nodes))

	return dfsRecursive(g.nodes[startIndex], val, &visited, &pathOut), &pathOut
}

// dfsRecursive returns true if the value is found and false if the value is not
func dfsRecursive(n *node, val int, visited *[]bool, pathOut *[]int) bool {
	(*visited)[n.index] = true
	if n.val == val {
		*pathOut = append(*pathOut, n.index)
		return true
	}

	for _, currentNeighbor := range n.neighbors {
		if !(*visited)[currentNeighbor.n.index] {
			if dfsRecursive(currentNeighbor.n, val, visited, pathOut) {
				*pathOut = append(*pathOut, n.index)
				return true
			}
		}
	}

	return false
}

type dfsShared struct {
	visited []bool
	// found represents the path ID (thread ID - 1) of whoever found the solution node.
	// -1 means the solution has not been found
	found int
	sync.Mutex
	sync.WaitGroup
}

// dfsMultithreaded finds a value in a graph using a number of simultaneous DFS searches with a shared visited list.
// dfsMultithreaded knows whether a value exists in the maze but doesn't know a unified path from the start to the end.
// exists is an index which specifies which search ended up finding the value in the paths array.
// If exists is -1, there is valid path to the solution from any starting index.
func dfsMultithreaded(g *graph, val int, startIndecies []int) (exists bool, p *[][]int) {
	pathsOut := make([][]int, len(startIndecies), len(startIndecies))

	visitedArray := make([]bool, len(g.nodes), len(g.nodes))
	dfsData := dfsShared{
		visited: visitedArray,
		found:   -1,
	}

	for i, start := range startIndecies {
		pathsOut[i] = make([]int, 0)
		dfsData.Add(1)
		go dfsRecursiveSynchronizer(g.nodes[start], val, &dfsData, &pathsOut[i], i)
	}

	dfsData.Wait()
	return dfsData.found != -1, &pathsOut
}

func dfsRecursiveSynchronizer(n *node, val int, dfsData *dfsShared, myPath *[]int, index int) {
	defer dfsData.Done()
	dfsRecursiveMultithreaded(n, val, dfsData, myPath, index)
}

// dfsRecursive returns true if the value is found and false if the value is not.
// It writes to pathsOut its solution based on the index passed in from dfsRecursive.
func dfsRecursiveMultithreaded(n *node, val int, dfsData *dfsShared, myPath *[]int, index int) bool {
	dfsData.Lock()
	// End the search if another path found the target value.
	if dfsData.found != -1 {
		dfsData.Unlock()
		return true
	}
	// End the search if this node has already been claimed.
	if (*dfsData).visited[n.index] == true {
		dfsData.Unlock()
		return false
	}
	// End the search if this node contains the target value.
	if n.val == val {
		dfsData.found = index
		dfsData.Unlock()
		return true
	}
	// Otherwise, claim the node.
	(*dfsData).visited[n.index] = true
	dfsData.Unlock()

	// Append the path as it goes on, not in reverse, to show all searching strands.
	*myPath = append(*myPath, n.index)

	for _, currentNeighbor := range n.neighbors {
		if dfsRecursiveMultithreaded(currentNeighbor.n, val, dfsData, myPath, index) {
			return true
		}
	}

	return false
}

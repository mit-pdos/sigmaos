package main

import (
	"context"
	"sync"
	"time"
)

// Time to run Multithreaded BFS before timing out.
const timeoutMilliseconds = 1000

func GetSeekerLocations(m *maze, numSeekers int) []int {
	// Math to place evenly spaced seekers in the middle of the maze
	spacerForIndex := m.width / numSeekers
	rowForIndex := m.height*m.width/2 - spacerForIndex/2
	starts := make([]int, numSeekers, numSeekers)

	for i := 0; i < numSeekers; i++ {
		starts[i] = rowForIndex + spacerForIndex*i
	}
	return starts
}

// DFS finds the first node with a given value and returns:
// - a boolean which is true if the value is accessible
// - a slice of indexes with the order of nodes to get there, starting with the node of the desired value and ending with the starting node
func DFS(g *graph, val int, startIndex int) (exists bool, path *[]int) {
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

// DFSMultithreaded finds a value in a graph using a number of simultaneous DFS searches with a shared visited list.
// DFSMultithreaded knows whether a value exists in the maze but doesn't know a unified path from the start to the end.
// exists is an index which specifies which search ended up finding the value in the paths array.
// If exists is -1, there is valid path to the solution from any starting index.
func DFSMultithreaded(g *graph, val int, startIndecies []int) (exists int, p *[][]int) {
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
	return dfsData.found, &pathsOut
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

// BFS finds the first node with a given value and returns:
// - a boolean which is true if the value is accessible
// - a path slice of indexes covering everything the search algorithm covered, in the order they were visited
// - a solution slice of indexes with the order of nodes to efficiently get to the value, starting with the node of the desired value and ending with the starting node
func BFS(g *graph, val int, startIndex int) (exists bool, path *[]int, solution *[]int) {
	pathOut := make([]int, 0)
	solutionOut := make([]int, 0)
	visited := make([]bool, len(g.nodes), len(g.nodes))
	parents := make([]int, len(g.nodes), len(g.nodes))
	// Arbitrary buffer size
	queue := make(chan int, 1000)

	queue <- startIndex
	visited[startIndex] = true
	parents[startIndex] = -1

	success, valIndex := bfsRecursive(g, queue, val, &visited, &parents, &pathOut)

	// Backtrack through parents to find the shortest path.
	if success {
		// Start at the goal node.
		i := valIndex
		for i != -1 {
			solutionOut = append(solutionOut, i)
			i = parents[i]
		}
	}

	// Cut off the part of the path that overwrites the solution
	pathOut = pathOut[:len(pathOut)-1]
	return success, &pathOut, &solutionOut
}

func bfsRecursive(g *graph, queue chan int, val int, visited *[]bool, parents *[]int, pathOut *[]int) (success bool, valIndex int) {
	if len(queue) == 0 {
		return false, -1
	}

	currentNode := g.nodes[<-queue]
	*pathOut = append(*pathOut, currentNode.index)

	if currentNode.val == val {
		return true, currentNode.index
	}

	for _, currentNeighbor := range currentNode.neighbors {
		if !(*visited)[currentNeighbor.n.index] {
			(*visited)[currentNeighbor.n.index] = true
			queue <- currentNeighbor.n.index
			(*parents)[currentNeighbor.n.index] = currentNode.index
		}
	}

	ok, index := bfsRecursive(g, queue, val, visited, parents, pathOut)
	if ok {
		return true, index
	}

	return false, -1
}

func BFSIterative(g *graph, val int, startIndex int) (exists bool, path *[]int, solution *[]int) {
	pathOut := make([]int, 0)
	solutionOut := make([]int, 0)
	visited := make([]bool, len(g.nodes), len(g.nodes))
	parents := make([]int, len(g.nodes), len(g.nodes))
	// Arbitrary buffer size
	queue := make(chan int, 1000)

	queue <- startIndex
	visited[startIndex] = true
	parents[startIndex] = -1

	success := false
	valIndex := -1

	for len(queue) != 0 {
		currentNode := g.nodes[<-queue]
		pathOut = append(pathOut, currentNode.index)

		if currentNode.val == val {
			success = true
			valIndex = currentNode.index
			break
		}

		for _, currentNeighbor := range currentNode.neighbors {
			if !visited[currentNeighbor.n.index] {
				visited[currentNeighbor.n.index] = true
				queue <- currentNeighbor.n.index
				parents[currentNeighbor.n.index] = currentNode.index
			}
		}
	}

	// Backtrack through parents to find the shortest path.
	if success {
		// Start at the goal node.
		i := valIndex
		for i != -1 {
			solutionOut = append(solutionOut, i)
			i = parents[i]
		}
	}

	return success, &pathOut, &solutionOut
}

// Multithreaded BFS has a thread manager and a number of senders.
// The thread manager starts a number of senders.
// The senders share an input and output channel.
// The thread manager sends new indexes to check into the input channel, based on the outputs.
// The senders send back the neighbors of the indexes in the input channel.
// The thread manager analyzes the output of the senders to determine if nodes are visited
// If they are not, it will put them into the parents array and back into the input queue.
// Once the solution is found, the thread manager will close the input queue which kills the senders.
// The thread manager will calculate the solution path and return.
// If there are no more items in the channel that the senders output into and it's been more than a preset time, the BFSMultithreaded cancels, because either there is no solution or the maze is too big.

type childParentPair struct {
	parent   int
	child    int
	threadID int
}

// BFSMultithreaded returns references to the success, the paths array, and the solution array
func BFSMultithreaded(g *graph, goalVal int, startIndex int, maxThreads int) (bool, *[][]int, *[]int) {
	// init
	// Channels have arbitrary buffer sizes - maybe they should be the size of maxThreads?
	parentIn := make(chan int, 1000)
	childOut := make(chan childParentPair, 1000)
	visited := make([]bool, len(g.nodes), len(g.nodes))
	parents := make([]int, len(g.nodes), len(g.nodes))
	paths := make([][]int, maxThreads, maxThreads)
	var tracker sync.WaitGroup

	childOut <- childParentPair{parent: -1, child: g.nodes[startIndex].index}

	ctx, cancel := context.WithCancel(context.Background())
	for i := 0; i < maxThreads; i++ {
		tracker.Add(1)
		go bfsThread(ctx, g, parentIn, childOut, goalVal, &tracker, i)
	}

	indexOfGoalNode := -1
	timeStart := time.Now().UnixMilli()
	for {
		// Don't get stuck waiting for an output that will never come
		if len(childOut) == 0 {
			// Arbitrarily end after a predetermined amount of time
			// Otherwise, the function will hang indefinitely
			// This is because there is currently no way to tell if threads are in progress.
			if time.Now().UnixMilli()-timeStart > timeoutMilliseconds {
				cancel()
				close(parentIn)
				break
			}
		} else {
			pair := <-childOut

			if pair.child == -1 {
				// Terminate
				indexOfGoalNode = pair.parent
				cancel()
				close(parentIn)
				// Cut off the part of the path that overwrites the solution
				paths[pair.threadID] = paths[pair.threadID][:len(paths[pair.threadID])-1]
				break
			}
			if !visited[pair.child] {
				visited[pair.child] = true
				parents[pair.child] = pair.parent
				paths[pair.threadID] = append(paths[pair.threadID], pair.child)
				parentIn <- pair.child
			}
		}
	}
	// Cleanup
	tracker.Wait()
	close(childOut)

	solution := make([]int, 0)
	if indexOfGoalNode != -1 {
		// Backtrack to find the solution
		i := indexOfGoalNode
		for i != -1 {
			solution = append(solution, i)
			i = parents[i]
		}
	}
	return indexOfGoalNode != -1, &paths, &solution
}

func bfsThread(ctx context.Context, g *graph, parentIn chan int, childOut chan childParentPair, val int, tracker *sync.WaitGroup, Id int) {
	defer tracker.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case p, ok := <-parentIn:
			if !ok {
				return
			}

			currentNode := g.nodes[p]
			for _, currentNeighbor := range currentNode.neighbors {
				childOut <- childParentPair{parent: p, child: currentNeighbor.n.index, threadID: Id}
				// Check for termination after sending the value so the parents array knows where the solution is
				if currentNeighbor.n.val == val {
					// Terminate
					childOut <- childParentPair{parent: currentNeighbor.n.index, child: -1, threadID: Id}
				}
			}
		}
	}
}

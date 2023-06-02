package maze

import (
	"context"
	"sync"
	"time"
)

// Time to run Multithreaded BFS before timing out.
const timeoutMilliseconds = 1000

// bfs finds the first node with a given value and returns:
// - a boolean which is true if the value is accessible
// - a path slice of indexes covering everything the search algorithm covered, in the order they were visited
// - a solution slice of indexes with the order of nodes to efficiently get to the value, starting with the node of the desired value and ending with the starting node
func bfs(g *graph, val int, startIndex int) (exists bool, path *[][]int, solution *[]int) {
	pathsOut := make([][]int, 0)
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
	pathsOut = append(pathsOut, pathOut)
	return success, &pathsOut, &solutionOut
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

func bfsIterative(g *graph, val int, startIndex int) (exists bool, path *[]int, solution *[]int) {
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

// bfsMultithreaded returns references to the success, the paths array, and the solution array
func bfsMultithreaded(g *graph, goalVal int, startIndex int, maxThreads int) (bool, *[][]int, *[]int) {
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

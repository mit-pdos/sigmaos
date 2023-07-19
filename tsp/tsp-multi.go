package tsp

import (
	"fmt"
	"sync"
)

// TSPMulti is a multithreaded implementation of TSP, which
// finds the shortest cycle from homeNode to homeNode, passing
// through every node in the graph exactly once.
// TSPMulti returns the length of the shortest path, the order
// of the shortest path, and any error.
// The number of threads are equal to the number of cities
// to the power of depthToFork.
func (g *Graph) TSPMulti(homeNode int, depthToFork int) (int, []int, error) {
	if depthToFork < 0 {
		return -1, nil, mkErr("depthToFork must be non-negative")
	}
	if depthToFork == 0 {
		// anything to the power of 0 is 1
		fmt.Printf("depthToFork is 0, calling TSPSingle\n")
		return g.TSPSingle(homeNode)
	}
	if depthToFork >= len(*g) {
		// depthToFork is too high to ever parallelize
		fmt.Printf("dpethToFork is too high, calling TSPSingle\n")
		return g.TSPSingle(homeNode)
	}
	// Init parent set with every node
	nodeSet := make(Set, 0)
	for i := range *g {
		if i != homeNode {
			nodeSet.Add(i)
		}
	}
	return tspMultiRecursive(g, homeNode, nodeSet, homeNode, depthToFork)
}

func tspMultiRecursive(g *Graph, homeNode int, choices Set, currentNode int, depthToFork int) (int, []int, error) {
	if len(choices) == 0 {
		return -1, nil, mkErr("tspMultiRecursive Failed: Invalid choices length")
	}

	minLen := INFINITY
	var minPath []int

	if len(choices) == 1 {
		minPath = make([]int, 2)
		minPath[0] = homeNode
		minPath[1] = choices[0]
		minLen = g.getEdge(homeNode, choices[0])
	} else {

		if depthToFork == 0 {
			// These do not need concurrency handling because all threads
			// will write to different indecies in the array
			minLengths := make([]int, len(choices))
			minPaths := make([][]int, len(choices))
			errors := make([]error, len(choices))
			wg := sync.WaitGroup{}
			for index, choice := range choices {
				wg.Add(1)
				go func(minLengths *[]int, minPaths *[][]int, errors *[]error, index int, choice int, wg *sync.WaitGroup) {
					(*minLengths)[index], (*minPaths)[index], (*errors)[index] = tspSingleRecursive(g, homeNode, choices.DelCopy(choice), choice)
					// error checking is handled outside
					wg.Done()
				}(&minLengths, &minPaths, &errors, index, choice, &wg)
			}
			wg.Wait()
			for _, err := range errors {
				// Arbitrarily returns the first error
				// XXX return all errors?
				if err != nil {
					return -1, nil, err
				}
			}
			for choice, length := range minLengths {
				if length < minLen {
					minLen = length
					minPath = minPaths[choice]
				}
			}
		} else {
			// Call a recursion for every possible choice
			depthToFork--
			for _, choice := range choices {
				length, path, err := tspMultiRecursive(g, homeNode, *choices.DelCopy(choice), choice, depthToFork)
				if err != nil {
					return -1, nil, err
				}

				if length < minLen {
					minLen = length
					minPath = path
				}
			}
		}
	}
	if currentNode == minPath[len(minPath)-1] {
		return -1, nil, mkErr("Impossible condition in tspMultiRecursive: node traversed to itself")
	}
	// Append this node to the search
	minLen += g.getEdge(minPath[len(minPath)-1], currentNode)
	minPath = append(minPath, currentNode)
	//fmt.Printf("%v%v Passing back %v at len %v\n", strings.Repeat("\t", len(*g)-1-len(choices)), currentNode, minPath, minLen)
	return minLen, minPath, nil
}

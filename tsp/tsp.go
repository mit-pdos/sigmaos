package tsp

// TSPSingle is a singlethreaded implementation of TSP, which
// finds the shortest cycle from homeNode to homeNode, passing
// through every node in the graph exactly once.
// TSPSingle returns the length of the shortest path, the order
// of the shortest path, and any error.
func (g *Graph) TSPSingle(homeNode int) (int, []int, error) {
	// Init parent set with every node
	nodeSet := make(Set, 0)
	for i := range *g {
		if i != homeNode {
			nodeSet.Add(i)
		}
	}
	return tspSingleRecursive(g, homeNode, nodeSet, homeNode)
}

func tspSingleRecursive(g *Graph, homeNode int, choices Set, currentNode int) (int, []int, error) {
	//fmt.Printf("%v: %v\n", currentNode, choices)

	if len(choices) == 0 {
		return -1, nil, mkErr("tspSingleRecursive Failed: Invalid choices length")
	}

	minLen := INFINITY
	var minPath []int

	if len(choices) == 1 {
		minPath = make([]int, 2)
		minPath[0] = homeNode
		minPath[1] = choices[0]
		minLen = g.getEdge(homeNode, choices[0])
	} else {
		// Call a recursion for every possible choice
		for _, choice := range choices {
			length, path, err := tspSingleRecursive(g, homeNode, choices.Del(choice), choice)
			if err != nil {
				return -1, nil, err
			}

			if length < minLen {
				minLen = length
				minPath = path
			}
		}
	}
	if currentNode == minPath[len(minPath)-1] {
		return -1, nil, mkErr("Impossible condition in tspSingleRecursive: node traversed to itself")
	}
	// Append this node to the search
	minLen += g.getEdge(minPath[len(minPath)-1], currentNode)
	minPath = append(minPath, currentNode)
	return minLen, minPath, nil
}

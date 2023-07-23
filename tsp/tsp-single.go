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
	//return tspSingleRecursive(g, homeNode, &nodeSet, homeNode)
	return tspSingleRecursiveOLD(g, homeNode, &nodeSet, homeNode)
}

func tspSingleRecursiveOLD(g *Graph, homeNode int, choices *Set, currentNode int) (int, []int, error) {
	//fmt.Printf("%v: %v\n", currentNode, choices)
	if len(*choices) == 0 {
		return -1, nil, mkErr("tspSingleRecursiveOLD Failed: Invalid choices length")
	}

	minLen := INFINITY
	var minPath []int

	if len(*choices) == 1 {
		minPath = make([]int, 2, len(*g))
		minPath[0] = homeNode
		minPath[1] = (*choices)[0]
		minLen = g.getEdge(homeNode, (*choices)[0])
	} else {
		// Call a recursion for every possible choice
		for _, choice := range *choices {
			choices.DelInPlace(choice)
			length, path, err := tspSingleRecursiveOLD(g, homeNode, choices, choice)
			choices.Add(choice)

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
		return -1, nil, mkErr("tspSingleRecursiveOLD node traversed to itself")
	}
	// Append this node to the search
	minLen += g.getEdge(minPath[len(minPath)-1], currentNode)
	minPath = append(minPath, currentNode)
	//fmt.Printf("%v%v Passing back %v at len %v\n", strings.Repeat("\t", len(*g)-1-len(*choices)), currentNode, minPath, minLen)
	return minLen, minPath, nil
}

func tspSingleRecursive(g *Graph, homeNode int, choices *Set, currentNode int) (int, []int, error) {
	//fmt.Printf("%v: %v\n", currentNode, choices)
	if len(*choices) == 0 {
		return -1, nil, mkErr("tspSingleRecursive Failed: Invalid choices length")
	}

	minLen := INFINITY
	var minPath []int

	// Call a recursion for every possible choice
	for _, choice := range *choices {
		// Don't copy - just edit
		var length int
		var path []int
		var err error
		if len(*choices) == 5 {
			length, path, err = tspSingleRecursiveBase(g, homeNode, choices)
		} else {
			choices.DelInPlace(choice)
			length, path, err = tspSingleRecursive(g, homeNode, choices, choice)
			choices.Add(choice)
		}
		if err != nil {
			return -1, nil, err
		}

		if length < minLen {
			minLen = length
			minPath = path
		}
	}

	if currentNode == minPath[len(minPath)-1] {
		return -1, nil, mkErr("tspSingleRecursive node traversed to itself")
	}
	// Append this node to the search
	minLen += g.getEdge(minPath[len(minPath)-1], currentNode)
	minPath = append(minPath, currentNode)
	//fmt.Printf("%v%v Passing back %v at len %v\n", strings.Repeat("\t", len(*g)-1-len(*choices)), currentNode, minPath, minLen)
	return minLen, minPath, nil
}

// This gets called at the second to final layer, so it will return data
// aggregated from 5! (120) cities. It assumes choices is size 5.
func tspSingleRecursiveBase(g *Graph, homeNode int, choices *Set) (int, []int, error) {
	minPath := make([]int, 6, len(*g))
	minPath[0] = homeNode

	minLen := INFINITY
	tmpLen := 0

	ephemeralChoices := make(Set, len(*choices))
	copy(ephemeralChoices, *choices)
	ephemeralChoices = append(ephemeralChoices, 0)

	for i1 := range ephemeralChoices {
		ephemeralChoices[i1], ephemeralChoices[len(ephemeralChoices)-1] = ephemeralChoices[len(ephemeralChoices)-1], ephemeralChoices[i1]
		for i2 := range ephemeralChoices {
			choices.DelInPlace(c2)
			for _, c3 := range ephemeralChoices {
				choices.DelInPlace(c3)
				for _, c4 := range ephemeralChoices {
					choices.DelInPlace(c4)
					for _, c5 := range ephemeralChoices {
						tmpLen = g.getEdge(homeNode, c5)
						tmpLen += g.getEdge(c5, c4)
						tmpLen += g.getEdge(c4, c3)
						tmpLen += g.getEdge(c3, c2)
						tmpLen += g.getEdge(c2, c1)

						if tmpLen < minLen {
							minLen = tmpLen
							minPath[1] = c5
							minPath[2] = c4
							minPath[3] = c3
							minPath[4] = c2
							minPath[5] = c1
						}
					}
					choices.Add(c4)
				}
				choices.Add(c3)
			}
			choices.Add(c2)
		}
		ephemeralChoices[i1], ephemeralChoices[len(ephemeralChoices)-1] = ephemeralChoices[len(ephemeralChoices)-1], ephemeralChoices[i1]
	}
	return minLen, minPath, nil
}

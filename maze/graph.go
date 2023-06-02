package maze

type neighbor struct {
	n      *node
	weight int
}

type node struct {
	val       int
	index     int
	neighbors []*neighbor
}

type graph struct {
	nodes []*node
}

func makeNode(v int, i int) *node {
	var newNode node
	newNode.val = v
	newNode.index = i
	return &newNode
}

// AddNode places a node at the end of the graph's slice of nodes
// and returns the index of the node in the slice.
func (g *graph) addNode(v int) int {
	g.nodes = append(g.nodes, makeNode(v, len(g.nodes)))
	return len(g.nodes) - 1
}

func (g *graph) setNode(index int, val int) {
	g.nodes[index].val = val
}

// addEdgeU is undirected and assumes all weights are 1
func addEdgeU(n1 *node, n2 *node) {
	n1.neighbors = append(n1.neighbors, &neighbor{n: n2, weight: 1})
	n2.neighbors = append(n2.neighbors, &neighbor{n: n1, weight: 1})
}

// addEdge assumes edges are not directed, adding edges to
// the other node for both i1 and i2
func (g *graph) addEdge(i1 int, i2 int) {
	// Avoid duplicate edges
	for _, adj := range g.nodes[i1].neighbors {
		if adj.n.index == i2 {
			return
		}
	}
	addEdgeU(g.nodes[i1], g.nodes[i2])
}

// removeEdgeU is unidirectional
func removeEdgeU(g *graph, i1 int, i2 int) {
	for i, adj := range g.nodes[i1].neighbors {
		if adj.n.index == i2 {
			g.nodes[i1].neighbors = append(g.nodes[i1].neighbors[:i], g.nodes[i1].neighbors[i+1:]...)
			break
		}
	}
}

// removeEdge bidirectional
func (g *graph) removeEdge(i1 int, i2 int) {
	removeEdgeU(g, i1, i2)
	removeEdgeU(g, i2, i1)
}

// hasEdge assumes bidirectional
func (g *graph) hasEdge(i1 int, i2 int) bool {
	for _, adj := range g.nodes[i1].neighbors {
		if adj.n.index == i2 {
			return true
		}
	}
	return false
}

// LEGACY PRINT FUNCTIONS
//
//	func getIndex(g *graph, n *node) int {
//		for i, node := range g.nodes {
//			if node == n {
//				return i
//			}
//		}
//		return -1
//	}
//
//	func (g *graph) Print() {
//		for nodeIndex, currentNode := range g.nodes {
//			// Index and value of the node
//			fmt.Printf("%v (%v) | [", nodeIndex, currentNode.val)
//			for _, adj := range currentNode.neighbors {
//				// Index and value of each neighbor
//				fmt.Printf("%v, ", getIndex(g, adj.n))
//			}
//			fmt.Print("]\n")
//		}
//	}

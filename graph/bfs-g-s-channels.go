package graph

import "time"

// Loosely based on https://people.eecs.berkeley.edu/~aydin/sc11_bfs.pdf

func BfsSingleChannels(g *Graph, n1 int, n2 int) (*[]int, error) {
	ts := time.Now()
	path, err := g.BfsSingleChannels(n1, n2)
	te := time.Now()
	printTime(ts, te, "BfsSingleChannels ran")
	return path, err
}

// BfsSingleChannels is a single-threaded, iterative breadth first search
// between two given nodes which works continuously via channels.
func (g *Graph) BfsSingleChannels(n1 int, n2 int) (*[]int, error) {
	if n1 == n2 {
		return &[]int{n1}, nil
	}
	if n1 > g.NumNodes-1 || n2 > g.NumNodes-1 || n1 < 0 || n2 < 0 {
		return nil, ERR_SEARCH_OOR
	}
	// p[index] gives the parent node of index
	p := make([]int, g.NumNodes)
	for i := range p {
		p[i] = NOT_VISITED
	}
	p[n1] = n1
	// XXX Replace with variable length queue
	// so all this memory doesn't need to be pre-allocated
	// Continual Set
	cs := make(chan int, g.NumEdges)
	CS := &cs

	*CS <- n1
	for len(*CS) > 0 {
		index := <-*CS
		adj := g.GetNeighbors(index)
		for _, a := range *adj {
			if p[a] == NOT_VISITED {
				*CS <- a
				p[a] = index
				if a == n2 {
					// Return the shortest path from n1 to n2
					return findPath(&p, n1, n2), nil
				}
			}
		}
	}
	return nil, ERR_NOPATH
}

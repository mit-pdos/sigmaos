package graph

import "time"

// Based on https://people.eecs.berkeley.edu/~aydin/sc11_bfs.pdf

func BfsSingleLayers(g *Graph, n1 int, n2 int) (*[]int, error) {
	ts := time.Now()
	path, err := g.bfsSingleLayers(n1, n2)
	te := time.Now()
	printTime(ts, te, "BfsSingleLayers ran")
	return path, err
}

// BfsSingleLayers is a single-threaded, iterative breadth first search
// between two given nodes which works in layers.
func (g *Graph) bfsSingleLayers(n1 int, n2 int) (*[]int, error) {
	if n1 == n2 {
		return &[]int{n1}, nil
	}
	if n1 > g.NumNodes()-1 || n2 > g.NumNodes()-1 || n1 < 0 || n2 < 0 {
		return nil, ERR_SEARCH_OOR
	}
	p := make([]int, g.NumNodes())
	for i := range p {
		p[i] = -1
	}
	p[n1] = n1
	level := 1
	// XXX Replace with variable length queue
	// so all this memory doesn't need to be pre-allocated
	// Frontier Set (current nodes)
	fs := make(chan int, g.NumEdges())
	FS := &fs
	// New Set (one hop away nodes)
	ns := make(chan int, g.NumEdges())
	NS := &ns

	*FS <- n1
	for len(*FS) > 0 || len(*NS) > 0 {
		for len(*FS) > 0 {
			index := <-*FS
			adj := g.GetNeighbors(index)
			for _, a := range *adj {
				if p[a] == -1 {
					*NS <- a
					p[a] = index
					if a == n2 {
						return findPath(&p, n1, n2), nil
					}
				}
			}
		}
		*FS = *NS
		ns = make(chan int, g.NumEdges())
		NS = &ns
		level++
	}
	return nil, ERR_NOPATH
}

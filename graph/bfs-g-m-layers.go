package graph

import (
	"context"
	"sync"
	"time"
)

// 1D Partitioning BFS scheme based on:
// https://ieeexplore.ieee.org/document/1559977
// https://people.eecs.berkeley.edu/~aydin/sc11_bfs.pdf

func BfsMultiLayers(g *Graph, n1 int, n2 int) (*[]int, error) {
	ts := time.Now()
	path, err := g.BfsMultiLayers(n1, n2)
	te := time.Now()
	printTime(ts, te, "BfsMultiLayers ran")
	return path, err
}

// BfsMultiLayers is a multi-threaded, 1D partitioned breadth first search
// between two given nodes which works in layers.
func (g *Graph) BfsMultiLayers(n1 int, n2 int) (*[]int, error) {
	// Each thread owns a set of nodes and their emanating edges.
	// Threads will go through all edges, add ones owned by themselves to their own NS,
	// and send ones owned by others to the others' NS, synchronized in layers.
	// Each thread keeps tracks of what's visited for themselves, and
	// the path is reconstructed at the end using everyone's parent slices.

	if n1 == n2 {
		return &[]int{n1}, nil
	}
	if n1 > g.NumNodes()-1 || n2 > g.NumNodes()-1 || n1 < 0 || n2 < 0 {
		return nil, ERR_SEARCH_OOR
	}

	// Frontier Sets (current nodes)
	FSs := make([]chan int, MAX_THREADS)
	// New Sets (one hop away nodes)
	NSs := make([]chan pair, MAX_THREADS)
	for i := 0; i < MAX_THREADS; i++ {
		FSs[i] = make(chan int, g.NumEdges())
		NSs[i] = make(chan pair, g.NumEdges())
	}
	NSs[getOwner(n1, MAX_THREADS)] <- pair{child: n1, parent: n1}

	graphs := g.partition(MAX_THREADS)
	parents := make([]map[int]int, MAX_THREADS)
	wg := sync.WaitGroup{}
	barrier := NewBarrier(MAX_THREADS)
	ctx, cancel := context.WithCancel(context.Background())
	// This defer should never matter, but is there just in case something goes wrong to avoid
	// a context leak.
	defer cancel()
	for j := 0; j < MAX_THREADS; j++ {
		i := j
		wg.Add(1)
		go func() {
			parents[i] = graphs[i].bfsMultiLayersThread(ctx, cancel, n2, FSs, NSs, i, barrier)
			wg.Done()
		}()
	}
	wg.Wait()
	return findPathPartitioned(&parents, n1, n2), nil
}

func (g *graphPartition) bfsMultiLayersThread(ctx context.Context, cancel context.CancelFunc, n2 int, FSs []chan int, NSs []chan pair, thread int, barrier *Barrier) map[int]int {
	//db.DPrintf(DEBUG_GRAPH, "Graph thread %v has partition %v", thread, g)
	p := make(map[int]int, 0)
	for key := range g.n {
		p[key] = -1
	}
	for {
		wg := sync.WaitGroup{}
		for len(FSs[thread]) > 0 {
			wg.Add(1)
			go g.pushNS(NSs, <-FSs[thread], &wg)
		}
		wg.Wait()
		barrier.Wait()
		if ctx.Err() != nil {
			return p
		}

		for len(NSs[thread]) > 0 {
			wg.Add(1)
			// XXX Make parents parallel safe and run this parallel
			pushFS(cancel, n2, FSs[thread], <-NSs[thread], &p, &wg)
		}
		wg.Wait()
		barrier.Wait()
		if ctx.Err() != nil {
			return p
		}
	}
}

func (g *graphPartition) pushNS(NSs []chan pair, node int, wg *sync.WaitGroup) {
	defer wg.Done()
	for _, neighbor := range g.n[node] {
		NSs[getOwner(neighbor, MAX_THREADS)] <- pair{child: neighbor, parent: node}
	}
}

func pushFS(cancel context.CancelFunc, n2 int, FS chan int, p pair, parents *map[int]int, wg *sync.WaitGroup) {
	defer wg.Done()
	if (*parents)[p.child] == -1 {
		(*parents)[p.child] = p.parent
		if p.child == n2 {
			cancel()
		}
		FS <- p.child
	}
}

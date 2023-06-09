package graph

import (
	"context"
	"sync"
	"time"
)

// 1D Partitioning BFS scheme loosely based on:
// https://ieeexplore.ieee.org/document/1559977
// https://people.eecs.berkeley.edu/~aydin/sc11_bfs.pdf

func BfsMultiChannels(g *Graph, n1 int, n2 int) (*[]int, error) {
	ts := time.Now()
	path, err := g.BfsMultiChannels(n1, n2)
	te := time.Now()
	printTime(ts, te, "BfsMultiChannels ran")
	return path, err
}

// BfsMultiChannels is a multi-threaded, 1D partitioned breadth first search
// between two given nodes which works in channels. The path returned is not
// guaranteed to be the shortest, but it will be valid.
func (g *Graph) BfsMultiChannels(n1 int, n2 int) (*[]int, error) {
	// Each thread owns a set of nodes and their emanating edges.
	// Threads will go through all edges, add ones owned by themselves to their own NS,
	// and send ones owned by others to the others' NS.
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
	ctx, cancel := context.WithCancel(context.Background())
	// This defer should never matter, but is there just in case something goes wrong to avoid
	//  a context leak.
	defer cancel()
	// Only adding 1 because the solution will only be found by one thread.
	for j := 0; j < MAX_THREADS; j++ {
		i := j
		wg.Add(1)
		go func() {
			parents[i] = graphs[i].bfsMultiChannelsThread(ctx, cancel, n2, FSs, NSs, i)
			wg.Done()
		}()
	}
	wg.Wait()
	// time.Sleep(1 * time.Millisecond)
	return findPathPartitioned(&parents, n1, n2), nil
}

// If there is no valid path from n1 to n2, bfsMultiChannelsThread will hang forever.
func (g *graphPartition) bfsMultiChannelsThread(ctx context.Context, cancel context.CancelFunc, n2 int, FSs []chan int, NSs []chan pair, thread int) map[int]int {
	// db.DPrintf(DEBUG_GRAPH, "Graph thread %v has partition %v", thread, g)
	p := make(map[int]int, 0)
	for key := range g.n {
		p[key] = -1
	}
	for {
		select {
		case <-ctx.Done():
			return p
		case index := <-NSs[thread]:
			if p[index.child] == -1 {
				// db.DPrintf(DEBUG_GRAPH, "|%v| Processing NSs %v", thread, index)
				p[index.child] = index.parent
				if index.child == n2 {
					// db.DPrintf(DEBUG_GRAPH, "|%v| Found solution %v from parent %v", thread, index.child, index.parent)
					cancel()
				}
				FSs[thread] <- index.child
			}
		case index := <-FSs[thread]:
			// db.DPrintf(DEBUG_GRAPH, "|%v| Processing FSs %v", thread, index)
			adj := g.getNeighbors(index)
			for _, a := range adj {
				NSs[getOwner(a, MAX_THREADS)] <- pair{child: a, parent: index}
			}
		}
	}
}

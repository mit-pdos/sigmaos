package graph

import (
	"errors"
	"math"
	"os"
	db "sigmaos/debug"
	"time"
)

//
// SEARCH UTILS
//

const NOT_VISITED = -1

const MAX_THREADS = 4

type pair struct {
	child  int
	parent int
}

// findPath finds the shortest path from n1 to n2.
func findPath(parents *[]int, n1 int, n2 int) *[]int {
	solution := make([]int, 0)
	i := n2
	for i != n1 {
		solution = append(solution, i)
		i = (*parents)[i]
	}
	solution = append(solution, n1)
	return &solution
}

func findPath64(parents *[]int, n1 int, n2 int) *[]int64 {
	solution := make([]int64, 0)
	i := n2
	for i != n1 {
		solution = append(solution, int64(i))
		i = (*parents)[i]
	}
	solution = append(solution, int64(n1))
	return &solution
}

func findPathPartitioned(parents *[]map[int]int, n1 int, n2 int) *[]int {
	solution := make([]int, 0)
	i := n2
	for i != n1 {
		solution = append(solution, i)
		i = (*parents)[getOwner(i, MAX_THREADS)][i]
	}
	solution = append(solution, n1)
	return &solution
}

//
// DEBUG / BENCH UTILS
//

const DEBUG_GRAPH = "GRAPH"
const PERF_GRAPH = DEBUG_GRAPH + "_PERF"

const (
	NOPATH      = "No valid path"
	ADDEDGE_OOR = "addEdge out of range"
	SAPPEND_NIL = "sAppend called with nil slice"
	SEARCH_OOR  = "searched indices out of range"
)

func mkErr(msg string) error {
	return errors.New("Graph: " + msg + "\n")
}

var (
	ERR_NOPATH      = mkErr(NOPATH)
	ERR_ADDEDGE_OOR = mkErr(ADDEDGE_OOR)
	ERR_SAPPEND_NIL = mkErr(SAPPEND_NIL)
	ERR_SEARCH_OOR  = mkErr(SEARCH_OOR)
)

func IsNoPath(e error) bool {
	if e == nil {
		return false
	}
	return (e.Error() == ERR_NOPATH.Error()) || (e.Error() == ERR_SEARCH_OOR.Error())
}

func printTime(timeStart time.Time, timeEnd time.Time, msg string) {
	// Manually calculate times from nanoseconds to have control over rounding
	timeEndNs := timeEnd.UnixNano() - timeStart.UnixNano()
	timeEndUs := float64(timeEndNs) / 1000.0
	timeEndMs := timeEndUs / 1000.0
	db.DPrintf(PERF_GRAPH, "%v in %.0f ms %.0f us\n", msg, timeEndMs, timeEndUs-(math.Floor(timeEndMs)*1000.0))
}

//
// PARTITIONING UTILS
//

type GraphPartition struct {
	// This is a map instead of a slice so that the key equals the index
	// of the node on the original graph.
	// XXX Make custom data structure which stores original int to avoid
	// wasting cache misses on a map.
	N        map[int][]int `json:"N"`
	NumNodes int           `json:"NumNodes"`
	NumEdges int           `json:"NumEdges"`
}

func (g *GraphPartition) getNeighbors(index int) []int {
	return (*g).N[index]
}

// partition naively partitions equal nodes to each thread.
// Partitions don't know the total number of nodes or edges.
// As a result, there may be edges to nodes that don't exist.
// XXX Add smart partitioning to ensure load balance between threads.
func (g *Graph) partition(numThreads int) []*GraphPartition {
	graphs := make([]*GraphPartition, numThreads)
	for i := 0; i < numThreads; i++ {
		graphs[i] = &GraphPartition{
			N:        make(map[int][]int, 0),
			NumNodes: 0,
			NumEdges: 0,
		}
	}
	for i := 0; i < g.NumNodes; i++ {
		partition := graphs[getOwner(i, numThreads)]
		partition.N[i] = *g.GetNeighbors(i)
		partition.NumNodes++
		partition.NumEdges += len(*g.N[i])
	}
	return graphs
}

func getOwner(index int, numThreads int) int {
	// For now, this is reliable without any data about
	// the graph since partitioning is not smart.
	return index % numThreads
}

//
// IMPORT UTILS
//

// XXX Make sure these are accessible in sigmaOS

const DATA_TINY_FN = "data/tiny.txt"

// FACEBOOK is from https://snap.stanford.edu/data/ego-Facebook.html
const DATA_FACEBOOK_FN = "data/facebook.txt"

// I run into problems using a graph this big, so for now I'm not testing with it.
// TWITCH is from https://snap.stanford.edu/data/twitch_gamers.html
const DATA_TWITCH_FN = "data/twitch.txt"

func ReadGraph(fn string) (*[]byte, error) {
	b, err := os.ReadFile(fn)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

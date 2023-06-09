package graph

import (
	"errors"
	"math"
	db "sigmaos/debug"
	"time"
)

const NOT_VISITED = -1

const DEBUG_GRAPH = "GRAPH"

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
	db.DPrintf(DEBUG_GRAPH, "%v in %.0f ms %.0f us\n", msg, timeEndMs, timeEndUs-(math.Floor(timeEndMs)*1000.0))
}

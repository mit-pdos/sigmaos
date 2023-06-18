package graph

import (
	"fmt"
	db "sigmaos/debug"
	"strconv"
	"strings"
)

// Graph helper functions

// Graph is a simple adjacency list implementation of a graph.
type Graph struct {
	N        []*[]int `json:"N"`
	NumNodes int      `json:"NumNodes"`
	NumEdges int      `json:"NumEdges"`
}

// Nodes contain the indecies of their neighbors rather than pointers
// to make importing and exporting data easier, since every level of
// BFS is followed by all-to-all communication.
// Graph assumes all edges are undirected and unweighted.

func (g *Graph) GetNeighbors(index int) *[]int {
	return (*g).N[index]
}

func initGraph(numNodes int) *Graph {
	g := Graph{
		N:        make([]*[]int, numNodes),
		NumNodes: numNodes,
		NumEdges: 0,
	}
	for i := range g.N {
		n := make([]int, 0)
		g.N[i] = &n
	}
	return &g
}

// XXX Add version which reads bytes
func ImportGraph(in string) (*Graph, error) {
	var err error
	var numNodes int
	strs := strings.Split(in, ";")
	if numNodes, err = strconv.Atoi(strs[0]); err != nil {
		return nil, err
	}

	g := initGraph(numNodes)

	for _, str := range strs[1:] {
		// Filter out EOF semicolons
		if len(str) < 2 {
			continue
		}
		var n1 int
		var n2 int
		edge := strings.Split(str, ",")
		if n1, err = strconv.Atoi(edge[0]); err != nil {
			return nil, err
		}
		if n2, err = strconv.Atoi(edge[1]); err != nil {
			return nil, err
		}
		if err = g.addEdge(n1, n2); err != nil {
			return nil, err
		}
	}
	return g, nil
}

// Export does NOT remove duplicate edges; every edge will appear twice.
func (g *Graph) Export() string {
	if g == nil {
		return ""
	}
	var out strings.Builder
	// Add the number of nodes
	out.WriteString(strconv.Itoa(g.NumNodes))
	out.WriteString(";")
	// Add all edges
	for i, adjacencies := range (*g).N {
		for _, adj := range *adjacencies {
			out.WriteString(strconv.Itoa(i))
			out.WriteString(",")
			out.WriteString(strconv.Itoa(adj))
			out.WriteString(";")
		}
	}
	// cut off trailing semicolon
	return out.String()[:out.Len()-1]
}

// addEdge assumes edges are sorted and inserts in sorted order
func (g *Graph) addEdge(n1 int, n2 int) error {
	if n1 > g.NumNodes || n1 < 0 {
		db.DPrintf(DEBUG_GRAPH, "addEdge out of range: %v in graph with %v nodes\n", n1, g.NumNodes)
		return ERR_ADDEDGE_OOR
	}
	if n2 > g.NumNodes || n1 < 0 {
		db.DPrintf(DEBUG_GRAPH, "addEdge out of range: %v in graph with %v nodes\n", n2, g.NumNodes)
		return ERR_ADDEDGE_OOR
	}

	var err error
	// addEdge trusts that there are no directed edges and arbitrarily assigns
	// the role of incrementing NumEdges to the first append.
	if err = sAppend((*g).N[n1], n2, &g.NumEdges); err != nil {
		return err
	}
	if err = sAppend((*g).N[n2], n1, nil); err != nil {
		return err
	}
	return nil
}

// XXX Replace with binary search and sorted insertion
func sAppend(s *[]int, new int, increment *int) error {
	if s == nil {
		db.DPrintf(DEBUG_GRAPH, "sAppend called with nil slice")
		return ERR_SAPPEND_NIL
	}
	// If edge is already in graph, silently ignore
	for _, val := range *s {
		if val == new {
			return nil
		}
	}
	*s = append(*s, new)
	if increment != nil {
		*increment++
	}
	return nil
}

func (g *Graph) Print() string {
	if g == nil {
		return ""
	}
	out := ""
	for i, adj := range (*g).N {
		out += fmt.Sprintf("(%v) %v\n", strconv.Itoa(i), adj)
	}
	return out
}

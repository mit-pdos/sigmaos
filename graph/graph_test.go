package graph_test

import (
	db "sigmaos/debug"
	"sigmaos/graph"
	"testing"

	"github.com/stretchr/testify/assert"
)

type searchAlg func(*graph.Graph, int, int) (*[]int, error)

func initGraph(t *testing.T, fn string) *graph.Graph {
	gd, err := graph.ReadGraph(fn)
	assert.Nil(t, err, "Failed to read graph data from file %v", fn)
	g, err := graph.ImportGraph(string(*gd))
	assert.Nil(t, err, "Failed to import graph")
	return g
}

func testAlg(t *testing.T, g *graph.Graph, alg searchAlg, n1 int, n2 int) {
	path, err := alg(g, n1, n2)
	if graph.IsNoPath(err) {
		db.DPrintf(graph.DEBUG_GRAPH, "No Valid Path from %v to %v in graph of size %v", n1, n2, g.NumNodes())
	} else {
		assert.Nil(t, err, "Failed to run BfsSingle()")
		db.DPrintf(graph.DEBUG_GRAPH, "Path from %v to %v: %v", n1, n2, path)
	}
}

func testAlgRepeated(t *testing.T, g *graph.Graph, alg searchAlg) {
	n1s := []int{-1, 0, 5, 0, 1, 3, 2}
	n2s := []int{0, 0, 5, 3, 3, 3420, 10000000}
	for i := range n1s {
		testAlg(t, g, alg, n1s[i], n2s[i])
	}
}

func TestImportExport(t *testing.T) {
	g := initGraph(t, graph.DATA_FACEBOOK_FN)
	// db.DPrintf(graph.DEBUG_GRAPH, g.Print())
	exp1 := g.Export()
	g2, err := graph.ImportGraph(exp1)
	assert.Nil(t, err, "Failed to import graph 2")
	exp2 := g2.Export()
	assert.Equal(t, exp1, exp2)
	db.DPrintf(graph.DEBUG_GRAPH, "NumNodes: %v, NumEdges: %v", g2.NumNodes(), g2.NumEdges())
}

func TestBFSSingleLayers(t *testing.T) {
	g := initGraph(t, graph.DATA_FACEBOOK_FN)
	testAlgRepeated(t, g, graph.BfsSingleLayers)
}

func TestBFSSingleChannels(t *testing.T) {
	g := initGraph(t, graph.DATA_FACEBOOK_FN)
	testAlgRepeated(t, g, graph.BfsSingleChannels)
}

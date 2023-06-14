package graph_test

import (
	"encoding/json"
	"path"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/graph"
	"sigmaos/graph/proto"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
	"sigmaos/rand"
	"sigmaos/test"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

//
// RAW GRAPH TESTS
//

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
		assert.Nil(t, err, "Failed to run search algorithm %v", alg)
		db.DPrintf(graph.DEBUG_GRAPH, "Path from %v to %v: %v", n1, n2, path)
	}
}

func testAlgRepeated(t *testing.T, g *graph.Graph, alg searchAlg) {
	// XXX Make better tests and actually check if the outputs are correct
	tests := [][]int{{-1, 0}, {0, 0}, {5, 5}, {0, 3}, {1, 3}, {3, 3420}, {508, 1080}, {217, 3456}, {2, 10000000}}
	for _, test := range tests {
		testAlg(t, g, alg, test[0], test[1])
		testAlg(t, g, alg, test[1], test[0])
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

func TestBFSMultiChannels(t *testing.T) {
	g := initGraph(t, graph.DATA_FACEBOOK_FN)
	testAlgRepeated(t, g, graph.BfsMultiChannels)
}

func TestBFSMultiLayers(t *testing.T) {
	g := initGraph(t, graph.DATA_FACEBOOK_FN)
	testAlgRepeated(t, g, graph.BfsMultiLayers)
}

//
// SIGMAOS GRAPH TESTS
//

type TstateGraph struct {
	*test.Tstate
	job string
	pid proc.Tpid
}

func makeTstateGraph(t *testing.T, j string) (*TstateGraph, error) {
	// Init
	tse := TstateGraph{}
	tse.job = j
	tse.Tstate = test.MakeTstateAll(t)
	var err error

	p := proc.MakeProc("graph", []string{strconv.FormatBool(test.Overlays)})
	p.SetNcore(proc.Tcore(1))
	if err = tse.Spawn(p); err != nil {
		db.DFatalf("|%v| Error spawning proc %v: %v", j, p, err)
		return nil, err
	}
	if err = tse.WaitStart(p.GetPid()); err != nil {
		db.DFatalf("|%v| Error waiting for proc %v to start: %v", j, p, err)
		return nil, err
	}
	db.DPrintf(graph.DEBUG_GRAPH, "|%v| Done with Initialization", j)
	tse.pid = p.GetPid()
	return &tse, nil
}

func TestServerInit(t *testing.T) {
	tsg, err := makeTstateGraph(t, rand.String(8))
	assert.Nil(t, err, "Failed to makeTstateGraph: %v", err)

	// Create an RPC client
	// XXX Get path from proc
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{tsg.FsLib}, path.Join(path.Join(graph.DIR_GRAPH, "g-server/")))
	assert.Nil(t, err, "ProtDevClnt creation failed: %v", err)

	// Import graph
	fn := graph.DATA_TINY_FN
	marshalled, err := json.Marshal(initGraph(t, fn))
	assert.Nil(t, err, "Failed to marshal graph at path %v: %v", fn, err)
	importArg := proto.GraphIn{Marshaled: marshalled}
	importRes := proto.GraphOut{}
	err = pdc.RPC("Graph.ImportGraph", &importArg, &importRes)
	assert.Nil(t, err, "Graph.ImportGraph failed with arg: %v and err: %v", importArg, err)

	// Run BFS
	bfsArg := proto.BfsInput{N1: 0, N2: 1}
	bfsRes := proto.Path{}
	err = pdc.RPC("Graph.RunBfs", &bfsArg, &bfsRes)
	assert.Nil(t, err, "Graph.RunBfs failed with arg: %v and err: %v", bfsArg, err)
	var p *[]int
	err = json.Unmarshal(bfsRes.GetMarshaled(), p)
	assert.Nil(t, err, "Failed to unmarshal path from arg %v: %v", bfsArg, err)
	db.DPrintf(graph.DEBUG_GRAPH, "Bfs Output: %v", p)

	tsg.Shutdown()
}

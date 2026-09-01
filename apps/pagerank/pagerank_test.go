package pagerank

import (
	"encoding/json"
	"path/filepath"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/test"
	"testing"

	"github.com/stretchr/testify/assert"

	sp "sigmaos/sigmap"

	db "sigmaos/debug"
)

func TestCompile(t *testing.T) {
}

func TestBasicGraph(t *testing.T) {
	// wikiGraph: the canonical PageRank example from Wikipedia, with nodes
	// A..K mapped to 1..11. Node 1 (A) has no outbound edges, so this graph
	// also exercises dangling-node handling.
	//
	//	B <-> C, D -> {A,B}, E -> {D,B,F}, F -> {B,E}, G -> {B,E}, H -> {B,E}, I -> {B,E}, J -> E, K -> E
	wikiGraph := []*GraphNode{
		{4, 1, 1}, {4, 2, 1}, // D -> A, B
		{3, 2, 1},                       // C  ->  B
		{2, 3, 1},                       // B  ->  C
		{5, 4, 1}, {5, 2, 1}, {5, 6, 1}, // E  ->  D, B, F
		{6, 2, 1}, {6, 5, 1}, // F  ->  B, E
		{7, 2, 1}, {7, 5, 1}, // G  ->  B, E
		{8, 2, 1}, {8, 5, 1}, // H  ->  B, E
		{9, 2, 1}, {9, 5, 1}, // I  ->  B, E
		{10, 5, 1}, // J  ->  E
		{11, 5, 1}, // K  ->  E
	}

	db.DPrintf(db.TEST, "Wiki Graph created")

	wikiGraphData, err := json.Marshal(wikiGraph)
	if !assert.Nil(t, err, "Error marshalling graph: %v", err) {
		return
	}

	db.DPrintf(db.TEST, "Marshalled to json: %s", string(wikiGraphData))

	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	db.DPrintf(db.TEST, "MultiRealmTstate created")

	err = mrts.GetRealm(test.REALM1).MkDir(filepath.Join(sp.S3, sp.LOCAL, "9ps3/pagerank"), 0777)
	if !assert.False(t, err != nil && !serr.IsErrorExists(err), "Error creating directory: %v", err) {
		return
	}

	db.DPrintf(db.TEST, "Directory created")

	in := filepath.Join(sp.S3, sp.LOCAL, "9ps3/pagerank/graph.json")
	_, err = mrts.GetRealm(test.REALM1).PutFile(in, 0777, sp.OWRITE, wikiGraphData)
	if !assert.Nil(t, err, "Error putting graph file: %v", err) {
		return
	}

	db.DPrintf(db.TEST, "Graph file created")

	out := filepath.Join(sp.S3, sp.LOCAL, "9ps3/pagerank/ranks.json")

	p := proc.NewProc("pagerank", []string{in, out, "0.85", "0.0000001"})
	err = mrts.GetRealm(test.REALM1).Spawn(p)
	if !assert.Nil(t, err, "Spawn") {
		return
	}

	db.DPrintf(db.TEST, "Pagerank process spawned")

	err = mrts.GetRealm(test.REALM1).WaitStart(p.GetPid())
	if !assert.Nil(t, err, "WaitStart error") {
		return
	}

	db.DPrintf(db.TEST, "Pagerank process started")

	status, err := mrts.GetRealm(test.REALM1).WaitExit(p.GetPid())
	if !assert.Nil(t, err, "WaitExit error %v", err) {
		return
	}
	if !assert.True(t, status.IsStatusOK(), "WaitExit status error: %v", status) {
		return
	}

	db.DPrintf(db.TEST, "Pagerank process exited OK")

	ranksData, err := mrts.GetRealm(test.REALM1).GetFile(out)
	if !assert.Nil(t, err, "Error getting ranks file: %v", err) {
		return
	}

	db.DPrintf(db.TEST, "Ranks file retrieved: %s", string(ranksData))

	var ranks []*PagerankResult
	err = json.Unmarshal(ranksData, &ranks)
	if !assert.Nil(t, err, "Error unmarshalling ranks: %v", err) {
		return
	}

	db.DPrintf(db.TEST, "Unmarshalled %d ranks", len(ranks))
	for _, r := range ranks {
		db.DPrintf(db.TEST, "  node %v rank %v", r.Node, r.Rank)
	}

	// Expected ranks for the wikiGraph (canonical Wikipedia PageRank example)
	// with damping factor 0.85. Values are the well-known reference results
	// (A 3.3%, B 38.4%, C 34.3%, D 3.9%, E 8.1%, F 3.9%, G..K 1.6%), so the
	// distribution sums to ~1.0. Nodes 1..11 map to A..K.
	expectedRanks := map[uint32]float64{
		1:  0.033, // A
		2:  0.384, // B
		3:  0.343, // C
		4:  0.039, // D
		5:  0.081, // E
		6:  0.039, // F
		7:  0.016, // G
		8:  0.016, // H
		9:  0.016, // I
		10: 0.016, // J
		11: 0.016, // K
	}

	// Tolerance of 0.001 accommodates the rounding in the reference values
	// above while still catching a genuinely wrong distribution.
	for _, rank := range ranks {
		expectedRank, exists := expectedRanks[rank.Node]
		if !assert.True(t, exists, "Unexpected node in ranks: %v", rank.Node) {
			return
		}
		if !assert.InDelta(t, expectedRank, rank.Rank, 0.001, "Rank mismatch for node %v", rank.Node) {
			return
		}
	}

	db.DPrintf(db.TEST, "All ranks within tolerance")
}

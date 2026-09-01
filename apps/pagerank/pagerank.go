package pagerank

import (
	"encoding/json"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"

	"github.com/alixaxel/pagerank"
)

type GraphNode struct {
	From   uint32 `json:"from"`
	To     uint32 `json:"to"`
	Weight int    `json:"weight"`
}

type PagerankResult struct {
	Node uint32  `json:"node"`
	Rank float64 `json:"rank"`
}

func Pagerank(args []string) {
	// First argument is input path
	// Second argument is output path
	// Third argument is damping factor
	// Fourth argument is convergence criteria

	db.DPrintf(db.PAGERANK, "Pagerank args: %v", args)

	pe := proc.GetProcEnv()
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error %v", err)
	}

	db.DPrintf(db.PAGERANK, "Pagerank client created")

	if err := sc.Started(); err != nil {
		db.DFatalf("Started error %v", err)
	}

	db.DPrintf(db.PAGERANK, "Pagerank client started")

	graphData, err := sc.GetFile(args[0])
	if err != nil {
		db.DFatalf("Error %v", err)
	}

	db.DPrintf(db.PAGERANK, "Graph data retrieved: %s", string(graphData))

	graph := pagerank.NewGraph()

	nodes := []*GraphNode{}

	json.Unmarshal(graphData, &nodes)

	for _, node := range nodes {
		graph.Link(node.From, node.To, float64(node.Weight))
	}

	dampingFactor, err := strconv.ParseFloat(args[2], 64)
	if err != nil {
		db.DFatalf("Error %v", err)
	}

	convergenceCriteria, err := strconv.ParseFloat(args[3], 64)
	if err != nil {
		db.DFatalf("Error %v", err)
	}

	result := []*PagerankResult{}

	graph.Rank(dampingFactor, convergenceCriteria, func(node uint32, rank float64) {
		resultRank := PagerankResult{
			Node: node,
			Rank: rank,
		}
		result = append(result, &resultRank)
	})

	resultData, err := json.Marshal(result)
	if err != nil {
		db.DFatalf("Error %v", err)
	}

	db.DPrintf(db.PAGERANK, "Pagerank result: %s", string(resultData))

	_, err = sc.PutFile(args[1], 0777, sp.OWRITE, resultData)
	if err != nil {
		db.DFatalf("Error %v", err)
	}

	db.DPrintf(db.PAGERANK, "Pagerank result written to %s", args[1])

	sc.ClntExitOK()
}

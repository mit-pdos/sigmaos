package tsp_test

import (
	"github.com/stretchr/testify/assert"
	db "sigmaos/debug"
	"sigmaos/tsp"
	"testing"
	"time"
)

func TestTSP(t *testing.T) {
	tspSolve, err := tsp.InitTSP(11, 10000, 10000)
	assert.Nil(t, err, "InitTSP Failed")
	err = tspSolve.GenTours()
	assert.Nil(t, err, "GenTours Failed")

	start := time.Now().UnixMilli()
	tspSolve.RunToursSingle()
	end := time.Now().UnixMilli()
	min, path := tspSolve.GetMinDist()
	db.DPrintf(tsp.DEBUG_TSP, "Minimum Distance by Single in %v ms: %v along path: %v", end-start, min, path)

	start = time.Now().UnixMilli()
	tspSolve.RunToursMulti()
	end = time.Now().UnixMilli()
	min, path = tspSolve.GetMinDist()
	db.DPrintf(tsp.DEBUG_TSP, "Minimum Distance by Multi in %v ms: %v along path: %v", end-start, min, path)
}

/*
func TestTSPMulti(t *testing.T) {
	tspSolve, err := tsp.InitTSP(11, 10000, 10000)
	assert.Nil(t, err, "InitTSP Failed")
	err = tspSolve.GenTours()
	assert.Nil(t, err, "GenTours Failed")
	tspSolve.RunToursMulti()
	min, path := tspSolve.GetMinDist()
	db.DPrintf(tsp.DEBUG_TSP, "Minimum Distance by Multi: %v along path: %v", min, path)
}
*/

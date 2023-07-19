package tsp_test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"runtime/pprof"
	"sigmaos/tsp"
	"testing"
	"time"
)

// Source: https://people.sc.fsu.edu/~jburkardt/datasets/tsp/tsp.html
var GRAPH_1 = tsp.Graph{
	{0, 3, 4, 2, 7},
	{3, 0, 4, 6, 3},
	{4, 4, 0, 5, 8},
	{2, 6, 5, 0, 6},
	{7, 3, 8, 6, 0}}
var PATH_1 = []int{0, 2, 1, 4, 3, 0}
var LENGTH_1 = 19

var GRAPH_2 = tsp.Graph{
	{0, 29, 82, 46, 68, 52, 72, 42, 51, 55, 29, 74, 23, 72, 46},
	{29, 0, 55, 46, 42, 43, 43, 23, 23, 31, 41, 51, 11, 52, 21},
	{82, 55, 0, 68, 46, 55, 23, 43, 41, 29, 79, 21, 64, 31, 51},
	{46, 46, 68, 0, 82, 15, 72, 31, 62, 42, 21, 51, 51, 43, 64},
	{68, 42, 46, 82, 0, 74, 23, 52, 21, 46, 82, 58, 46, 65, 23},
	{52, 43, 55, 15, 74, 0, 61, 23, 55, 31, 33, 37, 51, 29, 59},
	{72, 43, 23, 72, 23, 61, 0, 42, 23, 31, 77, 37, 51, 46, 33},
	{42, 23, 43, 31, 52, 23, 42, 0, 33, 15, 37, 33, 33, 31, 37},
	{51, 23, 41, 62, 21, 55, 23, 33, 0, 29, 62, 46, 29, 51, 11},
	{55, 31, 29, 42, 46, 31, 31, 15, 29, 0, 51, 21, 41, 23, 37},
	{29, 41, 79, 21, 82, 33, 77, 37, 62, 51, 0, 65, 42, 59, 61},
	{74, 51, 21, 51, 58, 37, 37, 33, 46, 21, 65, 0, 61, 11, 55},
	{23, 11, 64, 51, 46, 51, 51, 33, 29, 41, 42, 61, 0, 62, 23},
	{72, 52, 31, 43, 65, 29, 46, 31, 51, 23, 59, 11, 62, 0, 59},
	{46, 21, 51, 64, 23, 59, 33, 37, 11, 37, 61, 55, 23, 59, 0}}
var PATH_2 = []int{1, 13, 2, 15, 9, 5, 7, 3, 12, 14, 10, 8, 6, 4, 11, 1}
var LENGTH_2 = 291

func TestGraph(t *testing.T) {
	g, err := tsp.GenGraph(4, 10)
	assert.Nil(t, err, "GenGraph Failed")
	g.Print()
}

func TestTSPSingle1(t *testing.T) {
	GRAPH_1.Print()
	length, path, err := GRAPH_1.TSPSingle(0)
	assert.Nil(t, err, "TSPSingle Failed")
	assert.Equal(t, LENGTH_1, length)
	assert.Equal(t, PATH_1, path)
}

func TestTSPSingle2(t *testing.T) {
	length, path, err := GRAPH_2.TSPSingle(0)
	assert.Nil(t, err, "TSPSingle Failed")
	assert.Equal(t, LENGTH_2, length)
	assert.Equal(t, PATH_2, path)
}

func TestTSPMulti1(t *testing.T) {
	length, path, err := GRAPH_1.TSPMulti(0, 1)
	assert.Nil(t, err, "TSPMulti Failed")
	assert.Equal(t, LENGTH_1, length)
	assert.Equal(t, PATH_1, path)
}

func TestTSPMulti2(t *testing.T) {
	length, path, err := GRAPH_2.TSPMulti(0, 1)
	assert.Nil(t, err, "TSPMulti Failed")
	assert.Equal(t, LENGTH_2, length)
	assert.Equal(t, PATH_2, path)
}

func measureTSPSingle(t *testing.T, g *tsp.Graph) {
	start := time.Now().UnixMilli()
	length, path, err := g.TSPSingle(0)
	stop := time.Now().UnixMilli()
	assert.Nil(t, err, "TSPSingle Failed")
	fmt.Printf("TSPSingle found %v in %v ms via %v\n", length, stop-start, path)
}

func measureTSPMulti(t *testing.T, g *tsp.Graph, depthToFork int) {
	start := time.Now().UnixMilli()
	length, path, err := g.TSPMulti(0, depthToFork)
	stop := time.Now().UnixMilli()
	assert.Nil(t, err, "TSPSingle Failed")
	fmt.Printf("TSPMulti found %v in %v ms via %v\n", length, stop-start, path)
}

func TestTSPRandom(t *testing.T) {
	f, err := os.Create("cpu2.pprof")
	assert.Nil(t, err, "Pprof creation failed")
	g, err := tsp.GenGraph(12, 1000000)
	assert.Nil(t, err, "GenGraph Failed")
	//g.Print()
	err = pprof.StartCPUProfile(f)
	assert.Nil(t, err, "Pprof startup failed")
	measureTSPSingle(t, &g)
	pprof.StopCPUProfile()
	//measureTSPMulti(t, &g, 1)
}

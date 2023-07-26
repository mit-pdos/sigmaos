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
var GRAPH_5 = tsp.Graph{
	{0, 3, 4, 2, 7},
	{3, 0, 4, 6, 3},
	{4, 4, 0, 5, 8},
	{2, 6, 5, 0, 6},
	{7, 3, 8, 6, 0}}
var PATH_5 = []int{0, 2, 1, 4, 3, 0}
var LENGTH_5 = 21

// Randomly generated.
var GRAPH_13 = tsp.Graph{
	{148916, 811961, 558224, 638724, 808909, 210302, 663933, 489562, 157096, 706849, 750071, 557016, 732908},
	{811961, 997868, 818484, 915848, 924311, 273206, 526337, 48804, 209779, 776844, 654168, 920287, 627629},
	{558224, 818484, 245131, 571408, 729606, 196595, 891691, 749550, 573627, 289201, 87347, 486307, 499393},
	{638724, 915848, 571408, 551967, 75201, 149234, 491752, 888267, 509045, 82653, 843224, 205425, 609719},
	{808909, 924311, 729606, 75201, 100783, 8299, 548915, 341338, 417060, 12517, 520222, 546194, 748684},
	{210302, 273206, 196595, 149234, 8299, 769070, 794135, 263776, 219029, 615293, 510932, 569505, 651041},
	{663933, 526337, 891691, 491752, 548915, 794135, 616794, 283685, 281320, 470268, 867149, 640232, 538951},
	{489562, 48804, 749550, 888267, 341338, 263776, 283685, 632754, 571731, 997055, 322910, 609604, 746185},
	{157096, 209779, 573627, 509045, 417060, 219029, 281320, 571731, 480272, 652888, 264180, 960771, 823005},
	{706849, 776844, 289201, 82653, 12517, 615293, 470268, 997055, 652888, 219929, 581664, 381964, 298095},
	{750071, 654168, 87347, 843224, 520222, 510932, 867149, 322910, 264180, 581664, 604115, 625222, 210235},
	{557016, 920287, 486307, 205425, 546194, 569505, 640232, 609604, 960771, 381964, 625222, 616640, 713372},
	{732908, 627629, 499393, 609719, 748684, 651041, 538951, 746185, 823005, 298095, 210235, 713372, 281067}}
var PATH_13 = []int{0, 8, 1, 7, 6, 12, 10, 2, 5, 4, 9, 3, 11, 0}
var LENGTH_13 = 2598402

// Randomly generated.
var GRAPH_14 = tsp.Graph{
	{77, 58, 28, 44, 19, 33, 39, 76, 37, 55, 79, 89, 18, 20},
	{58, 68, 3, 89, 66, 86, 5, 1, 73, 83, 89, 40, 57, 96},
	{28, 3, 13, 83, 63, 65, 21, 48, 56, 24, 13, 1, 22, 78},
	{44, 89, 83, 18, 2, 42, 63, 7, 14, 4, 83, 67, 32, 52},
	{19, 66, 63, 2, 33, 68, 87, 62, 90, 33, 20, 4, 42, 25},
	{33, 86, 65, 42, 68, 58, 93, 53, 69, 75, 14, 44, 47, 31},
	{39, 5, 21, 63, 87, 93, 65, 96, 39, 78, 1, 35, 28, 8},
	{76, 1, 48, 7, 62, 53, 96, 50, 44, 53, 30, 55, 71, 22},
	{37, 73, 56, 14, 90, 69, 39, 44, 61, 28, 60, 80, 92, 79},
	{55, 83, 24, 4, 33, 75, 78, 53, 28, 17, 15, 46, 66, 68},
	{79, 89, 13, 83, 20, 14, 1, 30, 60, 15, 10, 4, 17, 94},
	{89, 40, 1, 67, 4, 44, 35, 55, 80, 46, 4, 37, 69, 85},
	{18, 57, 22, 32, 42, 47, 28, 71, 92, 66, 17, 69, 67, 88},
	{20, 96, 78, 52, 25, 31, 8, 22, 79, 68, 94, 85, 88, 38}}

// Source: https://people.sc.fsu.edu/~jburkardt/datasets/tsp/tsp.html
var GRAPH_15 = tsp.Graph{
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
var PATH_15 = []int{0, 12, 1, 14, 8, 4, 6, 2, 11, 13, 9, 7, 5, 3, 10, 0}
var LENGTH_15 = 291

func TestGraph(t *testing.T) {
	g, err := tsp.GenGraph(13, 100)
	assert.Nil(t, err, "GenGraph Failed")
	g.PrintExport()
}

func TestTSPSingle(t *testing.T) {
	length, path, err := GRAPH_13.TSPSingle(0)
	assert.Nil(t, err, "TSPSingle Failed")
	assert.Equal(t, LENGTH_13, length)
	assert.Equal(t, PATH_13, path)
}

func TestTSPMulti(t *testing.T) {
	length, path, err := GRAPH_13.TSPMulti(0, 1)
	assert.Nil(t, err, "TSPMulti Failed")
	assert.Equal(t, LENGTH_13, length)
	assert.Equal(t, PATH_13, path)
}

func measureTSPSingle(t *testing.T, g *tsp.Graph, suffix string) {
	f, err := os.Create("cpu_s" + suffix + ".pprof")
	assert.Nil(t, err, "File creation failed")
	err = pprof.StartCPUProfile(f)
	assert.Nil(t, err, "CPU pprof start failed")

	start := time.Now().UnixMilli()
	length, path, err := g.TSPSingle(0)
	stop := time.Now().UnixMilli()
	assert.Nil(t, err, "TSPSingle Failed")

	pprof.StopCPUProfile()
	f.Close()
	fmt.Printf("TSPSingle found %v in %v ms via %v\n", length, stop-start, path)
}

func measureTSPMulti(t *testing.T, g *tsp.Graph, suffix string, depthToFork int) {
	f, err := os.Create("cpu_m" + suffix + ".pprof")
	assert.Nil(t, err, "File creation failed")
	err = pprof.StartCPUProfile(f)
	assert.Nil(t, err, "CPU pprof start failed")

	start := time.Now().UnixMilli()
	length, path, err := g.TSPMulti(0, depthToFork)
	stop := time.Now().UnixMilli()
	assert.Nil(t, err, "TSPMulti Failed")

	pprof.StopCPUProfile()
	f.Close()
	fmt.Printf("TSPMulti found %v in %v ms via %v\n", length, stop-start, path)
}

func TestTSPProfile(t *testing.T) {
	g := GRAPH_13
	suffix := "_tmp"

	//measureTSPSingle(t, &g, suffix)
	measureTSPMulti(t, &g, suffix, 1)
}

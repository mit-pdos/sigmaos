package tsp_test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"runtime"
	"runtime/pprof"
	"sigmaos/tsp"
	"testing"
	"time"
)

// 5 Cities.
// Source: https://people.sc.fsu.edu/~jburkardt/datasets/tsp/tsp.html
var GRAPH_1 = tsp.Graph{
	{0, 3, 4, 2, 7},
	{3, 0, 4, 6, 3},
	{4, 4, 0, 5, 8},
	{2, 6, 5, 0, 6},
	{7, 3, 8, 6, 0}}
var PATH_1 = []int{0, 2, 1, 4, 3, 0}
var LENGTH_1 = 19

// 13 Cities. Randomly generated.
var GRAPH_2 = tsp.Graph{
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
var PATH_2 = []int{0, 8, 1, 7, 6, 12, 10, 2, 5, 4, 9, 3, 11, 0}
var LENGTH_2 = 2598402

// 15 Cities.
// Source: https://people.sc.fsu.edu/~jburkardt/datasets/tsp/tsp.html
var GRAPH_3 = tsp.Graph{
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
var PATH_3 = []int{0, 12, 1, 14, 8, 4, 6, 2, 11, 13, 9, 7, 5, 3, 10, 0}
var LENGTH_3 = 291

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

func TestTSPProfile(t *testing.T) {
	//g, err := tsp.GenGraph(13, 1000000)
	//assert.Nil(t, err, "GenGraph Failed")
	//g.Print()
	g := GRAPH_2
	suffix := "_old"
	fc, err := os.Create("cpu" + suffix + ".pprof")
	assert.Nil(t, err, "File creation failed")
	err = pprof.StartCPUProfile(fc)
	assert.Nil(t, err, "CPU pprof start failed")

	measureTSPSingle(t, &g)
	//measureTSPMulti(t, &g, 1)

	//fm, err := os.Create("memory" + suffix + ".pprof")
	//assert.Nil(t, err, "File creation failed")
	//err = pprof.WriteHeapProfile(fm)
	//assert.Nil(t, err, "Memory pprof failed")
	//fm.Close()

	pprof.StopCPUProfile()
	fc.Close()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("Alloc = %v MiB\n", m.Alloc/1024/1024)
}

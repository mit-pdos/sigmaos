package tsp_test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"sigmaos/tsp"
	"testing"
)

func TestGraph(t *testing.T) {
	g, err := tsp.GenGraph(4, 10)
	assert.Nil(t, err, "GenGraph Failed")
	g.Print()
}

func TestSet(t *testing.T) {
	s := tsp.Set{}
	s.Add(5)
	s.Add(2)
	s.Add(7)
	s.Add(8)
	s.Add(1)
	assert.Equal(t, true, s.Has(7))
	s = s.Del(7)
	assert.Equal(t, false, s.Has(7))
	assert.Equal(t, true, s.Has(5))
	s = s.Del(5)
	assert.Equal(t, false, s.Has(5))
	s = s.Del(7)
	s = s.Del(7)
	assert.Equal(t, false, s.Has(7))
	fmt.Printf("%v", s)
}

func TestTSP(t *testing.T) {
	g, err := tsp.GenGraph(11, 10000)
	assert.Nil(t, err, "GenGraph Failed")
	g.Print()
	length, path, err := g.TSPSingle(0)
	assert.Nil(t, err, "TSPSingle Failed")
	fmt.Printf("TSP Solved in %v by path: %v", length, path)
}

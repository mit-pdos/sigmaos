package tsp

import (
	"fmt"
	"math/rand"
	"time"
)

// Graph is an adjacency matrix
type Graph [][]int

func GenGraph(numCities int, maxDist int) (Graph, error) {
	if numCities > MAX_CITIES {
		return nil, mkErr("GenGraph Failed: Too Many Cities")
	}
	g := make(Graph, numCities)
	for i := range g {
		g[i] = make([]int, numCities)
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for n1 := range g {
		for n2 := range g[n1] {
			g.setEdge(n1, n2, r.Intn(maxDist))
		}
	}
	return g, nil
}

func (g *Graph) setEdge(n1 int, n2 int, weight int) {
	(*g)[n1][n2] = weight
	(*g)[n2][n1] = weight
}

// getEdge gets the wight of a given edge, which is its distance for TSP
func (g *Graph) getEdge(n1 int, n2 int) int {
	// Assumes the matrix is symmetrical and arbitrarily returns this side
	return (*g)[n1][n2]
}

func (g *Graph) Print() {
	fmt.Printf("%3v", "")
	for i := range *g {
		fmt.Printf("%10v", i)
	}
	fmt.Print("\n")
	for i1, _ := range *g {
		fmt.Printf("%3v", i1)
		for _, w2 := range (*g)[i1] {
			fmt.Printf("%10v", w2)
		}
		fmt.Print("\n")
	}
}

func (g *Graph) PrintExport() {
	fmt.Print("{")
	for i1, _ := range *g {
		for _, w2 := range (*g)[i1] {
			fmt.Printf("%v, ", w2)
		}
		fmt.Print("},\n{")
	}
	fmt.Print("}")

}

package tsp

import (
	"gonum.org/v1/gonum/stat/combin"
	"sync"
)

type TSP struct {
	c []City
	t []Tour
}

func InitTSP(numCities int, maxX int, maxY int) (TSP, error) {
	if numCities > 19 {
		return TSP{}, mkErr("InitTSP Failed: Too Many Cities")
	}
	tsp := TSP{}
	tsp.c = make([]City, numCities)
	for i := 0; i < numCities; i++ {
		tsp.c[i] = GenCity(maxX, maxY)
	}
	return tsp, nil
}

func (tsp *TSP) GenTours() error {
	var err error
	numTours := combin.NumPermutations(len(tsp.c), len(tsp.c))
	tsp.t = make([]Tour, numTours)
	gen := combin.NewPermutationGenerator(len(tsp.c), len(tsp.c))
	index := 0
	for gen.Next() {
		tsp.t[index], err = InitTour(&tsp.c, gen.Permutation(nil))
		if err != nil {
			return err
		}
		index++
	}
	return nil
}

// RunToursSingle assumes GenTours has already been run
func (tsp *TSP) RunToursSingle() {
	for i := range tsp.t {
		tsp.t[i].CalcDist()
	}
}

// RunToursMulti assumes GenTours has already been run
func (tsp *TSP) RunToursMulti() {
	wg := sync.WaitGroup{}
	for i := range tsp.t {
		wg.Add(1)
		i := i
		go func() {
			tsp.t[i].CalcDist()
			wg.Done()
		}()
	}
	wg.Wait()
}

// GetMinDist assumes RunToursSingle or RunToursMulti has already been run
// GetMinDist returns the minimum distance and cities in the closest order
func (tsp *TSP) GetMinDist() (float64, []City) {
	min := tsp.t[0].dist
	minIndex := 0
	for i, t := range tsp.t {
		if t.dist < min {
			min = t.dist
			minIndex = i
		}
	}
	// Put cities in the correct order
	o := tsp.t[minIndex].order
	out := make([]City, len(tsp.c))
	for i := 0; i < len(tsp.c); i++ {
		out[i] = tsp.c[o[i]]
	}
	return min, out
}

package main

import (
	"log"

	"ulambda/gg"
)

func main() {
	g := gg.MakeGraph()
	aDeps := []string{"b", "c"}
	g.AddThunk("a", aDeps, []string{})
	bDeps := []string{}
	g.AddThunk("b", bDeps, []string{})
	cDeps := []string{"b"}
	g.AddThunk("c", cDeps, []string{})
	thunks := g.GetThunks()
	for _, t := range thunks {
		log.Printf("Thunk %v\n", t)
	}
}

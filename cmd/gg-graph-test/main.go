package main

import (
	"log"

	"ulambda/gg"
)

func main() {
	g := gg.MakeGraph()
	aDeps := []string{"b", "c"}
	g.AddThunk("a", aDeps)
	bDeps := []string{}
	g.AddThunk("b", bDeps)
	cDeps := []string{"b"}
	g.AddThunk("c", cDeps)
	thunks := g.GetThunks()
	for _, t := range thunks {
		log.Printf("Thunk %v\n", t)
	}
}

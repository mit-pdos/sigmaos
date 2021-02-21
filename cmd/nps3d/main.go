package main

import (
	"ulambda/nps3"
)

func main() {
	nps3 := nps3.MakeNps3()
	nps3.Serve()
}

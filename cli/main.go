package main

import (
	"fmt"
	"log"
	sp "sigmaos/sigmap"
)

func main() {
    // Set Start to true
	fmt.Printf("start")

	dir := sp.NAMED
	ts, err1 := NewTstatePath(dir)
	if err1 != nil{
		fmt.Printf("error1")
	}
	sts, err := ts.GetDir(dir)
	if err != nil{
		fmt.Printf("error2")
	}
	log.Printf("%v: %v\n", dir, sp.Names(sts))

	defer ts.Shutdown()
}

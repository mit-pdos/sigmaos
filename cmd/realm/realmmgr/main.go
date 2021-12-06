package main

import (
	"log"
	"os"

	"ulambda/realm"
)

func main() {
	if len(os.Args) < 1 {
		log.Fatalf("Usage: %v", os.Args[0])
	}
	r := realm.MakeRealmMgr()
	r.Work()
}

package main

import (
	"log"

	"ulambda/mr"
	"ulambda/wc"
)

func main() {
	w := mr.MakeWorker(wc.Map, wc.Reduce)
	w.Work()
	log.Printf("Worker: finished\n")
}

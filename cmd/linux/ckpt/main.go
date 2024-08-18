package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"strconv"

	db "sigmaos/debug"

	"time"
)

// lazy-pages:
//  rm -rf dump; mkdir dump; ./bin/linux/ckpt 300
// sudo criu dump -vvvv --images-dir dump --shell-job --log-file log.txt -t $(pgrep ckpt)
// cp -r dump dump1
//   XXX shouldn't copy lazy pages
// ./bin/linux/lazy-pages dump1 dump1 2>&1 | tee out
//   or
// sudo criu lazy-pages -D dump1
// sudo criu restore -D dump1 --shell-job --lazy-pages --log-file restore.txt

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v <seconds> <npages>\n", os.Args[0])
		os.Exit(1)
	}

	db.DPrintf(db.ALWAYS, "Pid: %d", os.Getpid())

	s, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Printf("Atoi err %v\n", err)
		return
	}

	n, err := strconv.Atoi(os.Args[2])
	if err != nil {
		log.Printf("Atoi err %v\n", err)
		return
	}

	pagesz := os.Getpagesize()
	mem := make([]byte, pagesz*n)
	for i := 0; i < n; i++ {
		mem[i*pagesz] = byte(i)
	}

	timer := time.NewTicker(time.Duration(s) * time.Second)

	for {
		select {
		case <-timer.C:
			log.Println("!")
			return
		default:
			log.Print(".")
			r := rand.IntN(n)
			mem[r*pagesz] = byte(r)
			time.Sleep(2 * time.Second)
		}
	}
}

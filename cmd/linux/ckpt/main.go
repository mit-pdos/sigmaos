package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	db "sigmaos/debug"

	"time"
)

// lazy-pages:
//  rm -rf dump; mkdir dump; ./bin/linux/ckpt 300
// sudo criu dump -vvvv --images-dir dump --shell-job --log-file log.txt -t $(pgrep ckpt)
// cp -r dump dump1
// ./bin/linux/lazy-pages dump1 2>&1 | tee out
//   or
// sudo criu lazy-pages -D dump1
// sudo criu restore -D dump1 --shell-job --lazy-pages --log-file restore.txt

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v <sleep_length>\n", os.Args[0])
		os.Exit(1)
	}

	db.DPrintf(db.ALWAYS, "Pid: %d", os.Getpid())

	n, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Printf("Atoi err %v\n", err)
		return
	}

	timer := time.NewTicker(time.Duration(n) * time.Second)

	for {
		select {
		case <-timer.C:
			log.Println("!")
			return
		default:
			log.Print(".")
			time.Sleep(2 * time.Second)
		}
	}
}

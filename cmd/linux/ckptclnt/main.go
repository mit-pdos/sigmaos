package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	db "sigmaos/debug"

	"time"
)

// Run ckptsrv first:
//   sudo ./bin/linux/ckptsrv 100
// Then ckptclnt:
//   sudo ./bin/linux/ckptclnt 100
// Then in another shell:
//   sudo criu -vvvv dump --images-dir dump --shell-job --log-file log.txt -t 1473191
//   sudo criu restore -vvvv --images-dir dump --shell-job --log-file log-restore.txt

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v <sleep_length>\n", os.Args[0])
		os.Exit(1)
	}

	db.DPrintf(db.ALWAYS, "Pid: %d", os.Getpid())

	n, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Printf("Atoi err %v\n", err)
		return
	}
	timer := time.NewTicker(time.Duration(n) * time.Second)

	f, err := os.Create("/tmp/ckptclnt.txt")
	if err != nil {
		db.DFatalf("Error creating %v\n", err)
	}

	_, err = os.Open("/mnt/binfs/x")
	//_, err = os.Open("/mnt/binfs")
	if err != nil {
		db.DFatalf("open failed err %v\n", err)
	}

	// listOpenfiles()

	for {
		select {
		case <-timer.C:
			fmt.Println("!")
			f.Write([]byte("exiting"))
			return
		default:
			fmt.Print(".")
			f.Write([]byte("."))
			time.Sleep(2 * time.Second)
		}
	}
}

func listOpenfiles() {
	files, _ := ioutil.ReadDir("/proc")
	fmt.Println("listOpenfiles:")
	for _, f := range files {
		m, _ := filepath.Match("[0-9]*", f.Name())
		if f.IsDir() && m {
			fdpath := filepath.Join("/proc", f.Name(), "fd")
			ffiles, _ := ioutil.ReadDir(fdpath)
			for _, f := range ffiles {
				fpath, err := os.Readlink(filepath.Join(fdpath, f.Name()))
				if err != nil {
					fmt.Printf("listOpenfiles %v: err %v\n", f.Name(), err)
					continue
				}
				fmt.Printf("%v : %v\n", f, fpath)
			}
		}
	}
}

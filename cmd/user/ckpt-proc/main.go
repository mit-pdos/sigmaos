package main

import (
	"fmt"
	"io/ioutil"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"

	"time"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v <no/ext/self> <sleep_length> <npages>\n", os.Args[0])
		os.Exit(1)
	}
	cmd := os.Args[1]
	sec, err := strconv.Atoi(os.Args[2])
	if err != nil {
		db.DFatalf("Atoi error %v\n", err)
		return
	}
	npages, err := strconv.Atoi(os.Args[3])
	if err != nil {
		db.DFatalf("Atoi error %v\n", err)
		return
	}
	db.DPrintf(db.ALWAYS, "Running %v %d %d", cmd, sec, npages)

	var sc *sigmaclnt.SigmaClnt
	if cmd == "no" || cmd == "self" {
		sc, err = sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
		if err != nil {
			db.DFatalf("NewSigmaClnt error %v\n", err)
		}
		err = sc.Started()
		if err != nil {
			db.DFatalf("Started error %v\n", err)
		}
	}

	timer := time.NewTicker(time.Duration(sec) * time.Second)

	os.Stdin.Close() // XXX close in StartUproc

	if cmd == "ext" {
		syscall.Close(3) // close spproxyd.sock
	}

	f, err := os.Create("/tmp/sigmaos-perf/log.txt")
	if err != nil {
		db.DFatalf("Error creating %v\n", err)
	}

	listOpenfiles()

	pagesz := os.Getpagesize()
	mem := make([]byte, pagesz*npages)
	for i := 0; i < npages; i++ {
		mem[i*pagesz] = byte(i)
	}

	f.Write([]byte("."))

	if cmd == "self" {
		_, err := sc.Stat(sp.UX + "~any/")
		if err != nil {
			db.DFatalf("Stat err %v\n", err)
		}
		syscall.Close(4) // close spproxyd.sock
		f.Write([]byte("checkpointme...\n"))
		pn := sp.UX + "~any/" + sc.GetPID().String() + "/"
		if err := sc.CheckpointMe(pn); err != nil {
			db.DPrintf(db.ALWAYS, "CheckpointMe err %v\n", err)
			f.Write([]byte(fmt.Sprintf("CheckpointMe err %v\n", err)))
			sc.ClntExit(proc.NewStatusInfo(proc.StatusErr, "CheckpointMe failed", err))
			os.Exit(1)
		}
		f.Write([]byte("checkpointme done\n"))
		sc, err = sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
		f.Write([]byte(fmt.Sprintf("sigmaclnt err %v", err)))
		if err != nil {
			db.DFatalf("NewSigmaClnt err %v\n", err)
		}
	}

	for {
		select {
		case <-timer.C:
			f.Write([]byte("exit"))
			sc.ClntExitOK()
			return
		default:
			f.Write([]byte("."))
			r := rand.IntN(npages)
			mem[r*pagesz] = byte(r)
			time.Sleep(1 * time.Second)
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
				fmt.Printf("%v: %v : %v\n", f.Name(), f, fpath)
			}
		}
	}
}

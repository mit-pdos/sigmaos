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
	// "sigmaos/proc"
	// "sigmaos/sigmaclnt"
	// sp "sigmaos/sigmap"

	"time"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v <sleep_length> <npages>\n", os.Args[0])
		os.Exit(1)
	}
	sec, err := strconv.Atoi(os.Args[1])
	if err != nil {
		db.DFatalf("Atoi error %v\n", err)
		return
	}
	npages, err := strconv.Atoi(os.Args[1])
	if err != nil {
		db.DFatalf("Atoi error %v\n", err)
		return
	}
	db.DPrintf(db.ALWAYS, "Running %d %d", sec, npages)

	// sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	// if err != nil {
	// 	db.DFatalf("NewSigmaClnt error %v\n", err)
	// }
	// err = sc.Started()
	// if err != nil {
	// 	db.DFatalf("Started error %v\n", err)
	// }

	// pn := sp.UX + "~any/" + sc.GetPID().String() + "/"
	// _, err = sc.CheckpointMe(pn)
	// if err != nil {
	// 	db.DFatalf("Atoi error %v\n", err)
	// 	return
	// }

	timer := time.NewTicker(time.Duration(sec) * time.Second)

	// testDir := sp.S3 + "~any/fkaashoek/"
	//testDir := sp.UX + "~any/"
	//filePath := testDir + "example-out.txt"
	//fd, err := sc.Create(filePath, 0777, sp.OWRITE)
	//if err != nil {
	//db.DFatalf("Error creating out file in s3 %v\n", err)
	//}

	os.Stdin.Close()
	//syscall.Close(4) // close spproxyd.sock
	syscall.Close(3) // close spproxyd.sock
	//syscall.Close(3) // close ??
	//syscall.Close(8) // close ??

	f, err := os.Create("/tmp/sigmaos-perf/log.txt")
	if err != nil {
		db.DFatalf("Error creating %v\n", err)
	}

	// listOpenfiles()

	pagesz := os.Getpagesize()
	mem := make([]byte, pagesz*npages)
	for i := 0; i < npages; i++ {
		mem[i*pagesz] = byte(i)
	}

	for {
		select {
		case <-timer.C:
			f.Write([]byte("exit"))
			//sc.Write(fd, []byte("exiting"))
			//err = sc.CloseFd(fd)
			//sc.ClntExitOK()
			return
		default:
			f.Write([]byte("."))
			r := rand.IntN(npages)
			mem[r*pagesz] = byte(r)
			// sc.Write(fd, []byte("here sleep"))
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

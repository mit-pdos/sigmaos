package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	db "sigmaos/debug"
	//"sigmaos/proc"
	//"sigmaos/sigmaclnt"
	//sp "sigmaos/sigmap"

	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v <sleep_length>\n", os.Args[0])
		os.Exit(1)
	}

	db.DPrintf(db.ALWAYS, "Set started")

	// sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	// if err != nil {
	// 	db.DFatalf("NewSigmaClnt error %v\n", err)
	// }
	// err = sc.Started()
	// if err != nil {
	// 	db.DFatalf("Started error %v\n", err)
	// }

	n, err := strconv.Atoi(os.Args[1])
	if err != nil {
		db.DFatalf("Atoi error %v\n", err)
		return
	}

	timer := time.NewTicker(time.Duration(n) * time.Second)

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

	listOpenfiles()

	for {
		select {
		case <-timer.C:
			fmt.Println("exit")
			//sc.Write(fd, []byte("exiting"))
			//err = sc.CloseFd(fd)
			//sc.ClntExitOK()
			return
		default:
			fmt.Println("here sleep")
			// sc.Write(fd, []byte("here sleep"))
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
				fmt.Printf("%v: %v : %v\n", f.Name(), f, fpath)
			}
		}
	}
}

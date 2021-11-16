package fslambda

import (
	"io/ioutil"
	"log"
	"os"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/proc"
	"ulambda/procclnt"
)

type Downloader struct {
	pid  string
	src  string
	dest string
	*fslib.FsLib
	proc.ProcClnt
}

func MakeDownloader(args []string, debug bool) (*Downloader, error) {
	db.DPrintf("Downloader: %v\n", args)
	down := &Downloader{}
	down.pid = args[0]
	down.src = args[1]
	down.dest = args[2]
	fls := fslib.MakeFsLib("downloader")
	down.ProcClnt = procclnt.MakeProcClnt(fls)
	down.FsLib = fls
	down.Started(down.pid)
	return down, nil
}

func (down *Downloader) Work() {
	db.DPrintf("Downloading [%v] to [%v]\n", down.src, down.dest)
	contents, err := down.ReadFile(down.src)
	if err != nil {
		log.Printf("Downloader [pid: %v, src: %v, dest: %v] error...\n", down.pid, down.src, down.dest)
		log.Printf("Read download file error [%v]: %v\n", down.src, err)
	}
	err = ioutil.WriteFile(down.dest, contents, 0777)
	if err != nil {
		log.Printf("Couldn't write download file [%v]: %v\n", down.dest, err)
	}
	// Override umask
	err = os.Chmod(down.dest, 0777)
	if err != nil {
		log.Printf("Couldn't chmod newly downloaded file")
	}
}

func (down *Downloader) Exit() {
	down.Exited(down.pid, "OK")
}

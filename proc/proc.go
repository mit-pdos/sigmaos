package proc

import (
	"log"

	"ulambda/fs"
	"ulambda/fsrpc"
)

func Spawn(path string, fsc *FsClient) (path, err) {
	// write marshalled fds (incl. root) into "/proc/ctl"
	// return /proc/<pid>
}

func Getpid() int {
	fsclnt := fs.MakeFsClient()
	_, err := fsclnt.Open("/proc")
	if err != nil {
		log.Fatal("Open error", err)
	}
	// fsclnt.Write(fd, "Getpid")
	// buf := Read()
	// XXX convert buf into integer
	return 0
}

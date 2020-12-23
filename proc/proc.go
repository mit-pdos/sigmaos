package proc

import (
	"log"
	"strings"

	"ulambda/fs"
)

func Spawn(path string, clnt *fs.FsClient) (int, error) {
	fds := clnt.Lsof()
	log.Printf("fds %v\n", fds)
	fd, err := clnt.Create("/proc/spawn")
	if err != nil {
		log.Fatal("Open error:", err)
		return -1, err
	}
	_, err = clnt.Write(fd, []byte(path+" "+strings.Join(fds, " ")))
	if err != nil {
		log.Fatal("Write error: ", err)
		return -1, err
	}
	return 0, nil
}

func Getpid() int {
	// fsclnt := fs.MakeFsClient()
	//_, err := fsclnt.Open("/proc")
	//if err != nil {
	//	log.Fatal("Open error", err)
	//}
	// fsclnt.Write(fd, "Getpid")
	// buf := Read()
	// XXX convert buf into integer
	return 0
}

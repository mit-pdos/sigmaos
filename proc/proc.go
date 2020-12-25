package proc

import (
	"log"
	"strings"

	"ulambda/fs"
)

func Spawn(clnt *fs.FsClient, path string, fds []string) error {
	fd, err := clnt.Create("/proc/spawn")
	if err != nil {
		log.Fatal("Open error: ", err)
		return err
	}
	_, err = clnt.Write(fd, []byte(path+" "+strings.Join(fds, " ")))
	if err != nil {
		log.Fatal("Write error: ", err)
		return err
	}
	return nil
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

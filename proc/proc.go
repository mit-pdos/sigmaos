package proc

import (
	"encoding/json"
	"log"
	"math/rand"
	"strconv"

	"ulambda/fs"
)

func randPid(clnt *fs.FsClient) string {
	pid := rand.Int()
	return strconv.Itoa(pid)
}

// XXX close fds
func setAttr(clnt *fs.FsClient, pname string, key string, value []byte) error {
	fd, err := clnt.Open(pname + "/" + key)
	_, err = clnt.Write(fd, []byte(value))
	if err != nil {
		log.Fatal("Write error: ", err)
	}
	return err
}

// XXX clean up on failure
// XXX maybe one RPC?
func Spawn(clnt *fs.FsClient, program string, fids []string) (string, error) {
	pname := "/proc/" + randPid(clnt)
	_, err := clnt.Create(pname)
	if err != nil {
		log.Fatal("Create error: ", err)
		return "", err
	}
	err = setAttr(clnt, pname, "program", []byte(program))
	if err != nil {
		log.Fatal("setAttr error: ", err)
		return "", err
	}
	b, err := json.Marshal(fids)
	if err != nil {
		log.Fatal("Marshall error:", err)
	}
	err = setAttr(clnt, pname, "fds", b)
	if err != nil {
		log.Fatal("setAttr error: ", err)
		return "", err
	}
	err = setAttr(clnt, pname, "ctl", []byte("start"))
	return pname, nil
}

func Exit(clnt *fs.FsClient) error {
	return nil
}

func Wait(clnt *fs.FsClient, pname string) error {
	return nil
}

func Getpid(clnt *fs.FsClient, pname string) error {
	return nil
}

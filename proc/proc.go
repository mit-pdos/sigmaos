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
		log.Fatal("Spawn: create error: ", err)
		return "", err
	}
	err = setAttr(clnt, pname, "program", []byte(program))
	if err != nil {
		log.Fatal("Spawn: setAttr error: ", err)
		return "", err
	}
	fids = append([]string{pname}, fids...)
	b, err := json.Marshal(fids)
	if err != nil {
		log.Fatal("Spawn: marshall error: ", err)
	}
	err = setAttr(clnt, pname, "fds", b)
	if err != nil {
		log.Fatal("Spawn: setAttr error: ", err)
		return "", err
	}
	err = setAttr(clnt, pname, "ctl", []byte("start"))
	return pname, nil
}

func Exit(clnt *fs.FsClient) error {
	fd, err := clnt.Open(clnt.Proc + "/exit")
	if err != nil {
		log.Fatal("Exit: open error: ", err)
		return err
	}
	_, err = clnt.Write(fd, []byte("OK"))
	if err != nil {
		log.Fatal("Exit: write error: ", err)
		return err
	}
	return nil
}

func Wait(clnt *fs.FsClient, child string) ([]byte, error) {
	fd, err := clnt.Open(child + "/exit")
	if err != nil {
		log.Fatal("Wait: open error: ", err)
		return nil, err
	}
	b, err := clnt.Read(fd, 1024)
	if err != nil {
		log.Fatal("Wait: read error: ", err)
		return nil, err
	}
	return b, err
}

func Getpid(clnt *fs.FsClient) (string, error) {
	return clnt.Proc, nil
}

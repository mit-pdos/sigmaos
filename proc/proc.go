package proc

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"

	"ulambda/fs"
)

func randPid(clnt *fs.FsClient) string {
	pid := rand.Int()
	return strconv.Itoa(pid)
}

func makeAttr(clnt *fs.FsClient, fddir int, key string, value []byte) error {
	fd, err := clnt.CreateAt(fddir, key)
	if err != nil {
		return err
	}
	defer clnt.Close(fd)
	_, err = clnt.Write(fd, []byte(value))
	return err
}

func setAttr(clnt *fs.FsClient, fddir int, key string, value []byte) error {
	fd, err := clnt.OpenAt(fddir, key)
	if err != nil {
		return err
	}
	defer clnt.Close(fd)
	_, err = clnt.Write(fd, []byte(value))
	return err
}

// XXX clean up on failure
// XXX maybe one RPC?
func Spawn(clnt *fs.FsClient, program string, args []string, fids []string) (string, error) {
	pid := randPid(clnt)
	pname := "/proc/" + pid
	fd, err := clnt.MkDir(pname)
	if err != nil {
		return "", err
	}
	defer clnt.Close(fd)
	err = makeAttr(clnt, fd, "program", []byte(program))
	if err != nil {
		clnt.RmDir(pname)
		return "", err
	}

	args = append([]string{pname}, args...)
	b, err := json.Marshal(args)
	if err != nil {
		clnt.RmDir(pname)
		return "", err
	}
	err = makeAttr(clnt, fd, "args", b)
	if err != nil {
		clnt.RmDir(pname)
		return "", err
	}

	b, err = json.Marshal(fids)
	if err != nil {
		clnt.RmDir(pname)
		return "", err
	}
	err = makeAttr(clnt, fd, "fds", b)
	if err != nil {
		clnt.RmDir(pname)
		return "", err
	}
	if clnt.Proc != "" {
		err = clnt.Pipe(clnt.Proc + "/" + "exit" + pid)
		if err != nil {
			clnt.RmDir(pname)
			return "", err
		}
		err := clnt.SymlinkAt(fd, "parent", clnt.Proc)
		if err != nil {
			clnt.RmDir(clnt.Proc + "/" + "exit" + pid)
			clnt.RmDir(pname)
			return "", err
		}
	}

	// Start process
	err = setAttr(clnt, fd, "ctl", []byte("start"))
	if err != nil {
		clnt.RmDir(clnt.Proc + "/" + "exit" + pid)
		clnt.RmDir(pname)
		return "", err
	}
	return pname, err
}

func Exit(clnt *fs.FsClient, v ...interface{}) error {
	pid := filepath.Base(clnt.Proc)
	defer func() {
		clnt.RmDir(clnt.Proc)
		os.Exit(0)
	}()
	fd, err := clnt.Open(clnt.Proc + "/parent/exit" + pid)
	if err != nil {
		return err
	}
	defer clnt.Close(fd)
	_, err = clnt.Write(fd, []byte(fmt.Sprint(v...)))
	if err != nil {
		return err
	}
	return nil
}

func Wait(clnt *fs.FsClient, child string) ([]byte, error) {
	pid := filepath.Base(child)
	fd, err := clnt.Open(clnt.Proc + "/exit" + pid)
	if err != nil {
		return nil, err
	}
	defer clnt.Close(fd)
	defer clnt.Remove(clnt.Proc + "/exit" + pid)
	b, err := clnt.Read(fd, 1024)
	if err != nil {
		return nil, err
	}
	return b, err
}

func Getpid(clnt *fs.FsClient) (string, error) {
	return clnt.Proc, nil
}

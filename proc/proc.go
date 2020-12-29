package proc

import (
	"encoding/json"
	"log"
	"math/rand"
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
		log.Fatal("OpenAt error: ", err)
	}
	defer clnt.Close(fd)
	_, err = clnt.Write(fd, []byte(value))
	if err != nil {
		log.Fatal("Write error: ", err)
	}
	return err
}

func setAttr(clnt *fs.FsClient, fddir int, key string, value []byte) error {
	fd, err := clnt.OpenAt(fddir, key)
	if err != nil {
		log.Fatal("OpenAt error: ", err)
	}
	defer clnt.Close(fd)
	_, err = clnt.Write(fd, []byte(value))
	if err != nil {
		log.Fatal("Write error: ", err)
	}
	return err
}

// XXX clean up on failure
// XXX maybe one RPC?
func Spawn(clnt *fs.FsClient, program string, args []string, fids []string) (string, error) {
	pid := randPid(clnt)
	pname := "/proc/" + pid
	fd, err := clnt.MkDir(pname)
	if err != nil {
		log.Fatal("Spawn: mkdir error: ", err)
		return "", err
	}
	defer clnt.Close(fd)
	err = makeAttr(clnt, fd, "program", []byte(program))
	if err != nil {
		log.Fatal("Spawn: setAttr error: ", err)
		return "", err
	}

	args = append([]string{pname}, args...)
	b, err := json.Marshal(args)
	if err != nil {
		log.Fatal("Spawn: marshall error: ", err)
	}
	err = makeAttr(clnt, fd, "args", b)
	if err != nil {
		log.Fatal("Spawn: setAttr error: ", err)
		return "", err
	}

	b, err = json.Marshal(fids)
	if err != nil {
		log.Fatal("Spawn: marshall error: ", err)
	}
	err = makeAttr(clnt, fd, "fds", b)
	if err != nil {
		log.Fatal("Spawn: setAttr error: ", err)
		return "", err
	}

	log.Printf("clnt.Proc %v\n", clnt.Proc)
	if clnt.Proc != "" {
		err = clnt.Pipe(clnt.Proc + "/" + "exit" + pid)
		if err != nil {
			return "", err
		}
		err := clnt.SymlinkAt(fd, "parent", clnt.Proc)
		if err != nil {
			log.Fatal("Spawn: Symlink error: ", err)
		}
	}

	// Start process
	err = setAttr(clnt, fd, "ctl", []byte("start"))

	return pname, nil
}

func Exit(clnt *fs.FsClient) error {
	pid := filepath.Base(clnt.Proc)
	fd, err := clnt.Open(clnt.Proc + "/parent/exit" + pid)
	if err != nil {
		log.Fatal("Exit: open error: ", err)
		return err
	}
	defer clnt.Close(fd)
	_, err = clnt.Write(fd, []byte("OK"))
	if err != nil {
		log.Fatal("Exit: write error: ", err)
		return err
	}
	return nil
}

func Wait(clnt *fs.FsClient, child string) ([]byte, error) {
	pid := filepath.Base(child)
	fd, err := clnt.Open(clnt.Proc + "/exit" + pid)
	if err != nil {
		log.Fatal("Wait: open error: ", err)
		return nil, err
	}
	defer clnt.Close(fd)
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

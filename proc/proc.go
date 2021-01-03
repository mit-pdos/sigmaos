package proc

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"

	"ulambda/fsclnt"
	np "ulambda/ninep"
)

func randPid(clnt *fsclnt.FsClient) string {
	pid := rand.Int()
	return strconv.Itoa(pid)
}

func makeAttr(clnt *fsclnt.FsClient, fddir int, key string, value []byte) error {
	fd, err := clnt.CreateAt(fddir, key, 0, np.OWRITE)
	if err != nil {
		return err
	}
	defer clnt.Close(fd)
	_, err = clnt.Write(fd, 0, []byte(value))
	return err
}

func setAttr(clnt *fsclnt.FsClient, fddir int, key string, value []byte) error {
	fd, err := clnt.OpenAt(fddir, key, np.OWRITE)
	if err != nil {
		return err
	}
	defer clnt.Close(fd)
	_, err = clnt.Write(fd, 0, []byte(value))
	return err
}

// XXX clean up on failure
// XXX maybe one RPC?
func Spawn(clnt *fsclnt.FsClient, program string, args []string, fids []string) (string, error) {
	pid := randPid(clnt)
	pname := "name/procd/" + pid

	fd, err := clnt.Mkdir(pname, np.ORDWR)
	if err != nil {
		return "", err
	}
	defer clnt.Close(fd)
	err = makeAttr(clnt, fd, "program", []byte(program))
	if err != nil {
		//clnt.RmDir(pname)
		return "", err
	}

	args = append([]string{pname}, args...)
	b, err := json.Marshal(args)
	if err != nil {
		//clnt.RmDir(pname)
		return "", err
	}
	err = makeAttr(clnt, fd, "args", b)
	if err != nil {
		//clnt.RmDir(pname)
		return "", err
	}

	b, err = json.Marshal(fids)
	if err != nil {
		//clnt.RmDir(pname)
		return "", err
	}
	err = makeAttr(clnt, fd, "fds", b)
	if err != nil {
		//clnt.RmDir(pname)
		return "", err
	}
	if clnt.Proc != "" {
		err = clnt.Pipe(clnt.Proc+"/"+"exit"+pid, np.ORDWR)
		if err != nil {
			//clnt.RmDir(pname)
			return "", err
		}
		err := clnt.SymlinkAt(fd, "parent", clnt.Proc)
		if err != nil {
			//clnt.RmDir(clnt.Proc + "/" + "exit" + pid)
			//clnt.RmDir(pname)
			return "", err
		}
	}

	//fd, err := cons.clnt.Open("name/procd/makeproc", np.OWRITE)
	//if err != nil {
	//	log.Fatal("Open error: ", err)
	//}
	//_, err = cons.clnt.Write(fd1, 0, []byte("Hello world\n"))

	// Start process
	err = setAttr(clnt, fd, "ctl", []byte("start"))
	if err != nil {
		//clnt.RmDir(clnt.Proc + "/" + "exit" + pid)
		//clnt.RmDir(pname)
		return "", err
	}
	return pname, err
}

func Exit(clnt *fsclnt.FsClient, v ...interface{}) error {
	pid := filepath.Base(clnt.Proc)
	defer func() {
		//clnt.RmDir(clnt.Proc)
		os.Exit(0)
	}()
	fd, err := clnt.Open(clnt.Proc+"/parent/exit"+pid, np.OWRITE)
	if err != nil {
		return err
	}
	defer clnt.Close(fd)
	_, err = clnt.Write(fd, 0, []byte(fmt.Sprint(v...)))
	if err != nil {
		return err
	}
	return nil
}

func Wait(clnt *fsclnt.FsClient, child string) ([]byte, error) {
	pid := filepath.Base(child)
	fd, err := clnt.Open(clnt.Proc+"/exit"+pid, np.OREAD)
	if err != nil {
		return nil, err
	}
	// defer clnt.Remove(clnt.Proc + "/exit" + pid)
	defer clnt.Close(fd)
	b, err := clnt.Read(fd, 0, 1024)
	if err != nil {
		return nil, err
	}
	return b, err
}

func Getpid(clnt *fsclnt.FsClient) (string, error) {
	return clnt.Proc, nil
}

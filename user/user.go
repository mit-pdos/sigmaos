package user

import (
	"crypto/sha256"
	"fmt"
	"log"
	"path"

	// db "sigmaos/debug"
	"sigmaos/dbclnt"
	"sigmaos/fslib"
	np "sigmaos/ninep"
	"sigmaos/proc"
	"sigmaos/procclnt"
)

type User struct {
	Username string
	Password string
}

type UserLogin struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	input  []string
	dbc    *dbclnt.DbClnt
	pipefd int
}

func MkPassword(u string) string {
	p := u
	for j := 0; j < 10; j++ {
		p += u
	}
	s := sha256.Sum256([]byte(p))
	return fmt.Sprintf("%x", s)
}

func RunUserLogin(args []string) (*UserLogin, error) {
	ua := &UserLogin{}
	ua.FsLib = fslib.MakeFsLib("userlogin")
	ua.ProcClnt = procclnt.MakeProcClnt(ua.FsLib)
	ua.input = args[1:]
	dbc, err := dbclnt.MkDbClnt(ua.FsLib, np.DBD)
	if err != nil {
		return nil, err
	}
	ua.dbc = dbc
	ua.Started()
	return ua, nil
}

func (ua *UserLogin) writeResponse(data []byte) *proc.Status {
	_, err := ua.Write(ua.pipefd, data)
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("Pipe parse err %v\n", err), nil)
	}
	ua.Evict(proc.GetPid())
	return proc.MakeStatus(proc.StatusOK)
}

func (ua *UserLogin) Login() *proc.Status {
	log.Printf("login %v\n", ua.input[0])
	fd, err := ua.Open(path.Join(proc.PARENTDIR, proc.SHARED)+"/", np.OWRITE)
	if err != nil {
		return proc.MakeStatusErr(fmt.Sprintf("Open err %v\n", err), nil)
	}
	ua.pipefd = fd
	defer ua.Close(fd)
	var users []User
	q := fmt.Sprintf("SELECT * from user where username='%s';", ua.input[0])
	error := ua.dbc.Query(q, &users)
	if error != nil {
		return proc.MakeStatusErr(fmt.Sprintf("Query err %v", error), nil)
	}
	if len(users) == 0 {
		return proc.MakeStatusErr(fmt.Sprintf("Unknown user %v"), nil)
	}
	if users[0].Password != ua.input[1] {
		return proc.MakeStatusErr("Wrong password", nil)
	}
	log.Printf("login redirect\n")
	return proc.MakeStatusErr("Redirect", "/book/view/")
}

func (ua *UserLogin) Exit(status *proc.Status) {
	log.Printf("login status %v\n", status)
	ua.Exited(status)
}

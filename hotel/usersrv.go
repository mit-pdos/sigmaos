package hotel

import (
	"crypto/sha256"
	"fmt"
	"strconv"

	"sigmaos/dbclnt"
	np "sigmaos/ninep"
	"sigmaos/protdevsrv"
)

const (
	NUSER = 500
)

type UserRequest struct {
	Name     string
	Password string
}

type UserResult struct {
	OK string
}

type User struct {
	Username string
	Password string
}

type Users struct {
	dbc *dbclnt.DbClnt
}

func RunUserSrv(n string) error {
	u := &Users{}
	pds := protdevsrv.MakeProtDevSrv(np.HOTELUSER, u)
	dbc, err := dbclnt.MkDbClnt(pds.MemFs.FsLib, np.DBD)
	if err != nil {
		return err
	}
	u.dbc = dbc
	err = u.initDB()
	if err != nil {
		return err
	}
	return pds.RunServer()
}

func MkPassword(u string) string {
	p := u
	for j := 0; j < 10; j++ {
		p += u
	}
	s := sha256.Sum256([]byte(p))
	return fmt.Sprintf("%x", s)
}

func (s *Users) initDB() error {
	q := fmt.Sprintf("truncate user;")
	err := s.dbc.Exec(q)
	if err != nil {
		return err
	}
	for i := 0; i <= NUSER; i++ {
		u := "u_" + strconv.Itoa(i)
		p := MkPassword(u)
		q = fmt.Sprintf("INSERT INTO user (username, password) VALUES ('%v', '%v');", u, p)
		err = s.dbc.Exec(q)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Users) CheckUser(req UserRequest, res *UserResult) error {
	q := fmt.Sprintf("SELECT * from user where username='%s';", req.Name)
	var users []User
	error := s.dbc.Query(q, &users)
	res.OK = "False"
	if error != nil {
		return error
	}
	if len(users) == 0 {
		return fmt.Errorf("Unknown user %v", req.Name)
	}
	if req.Password != users[0].Password {
		return fmt.Errorf("Wrong password")
	}
	res.OK = "True"
	return nil
}

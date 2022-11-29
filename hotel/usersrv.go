package hotel

import (
	"crypto/sha256"
	"fmt"
	"strconv"

	"sigmaos/dbclnt"
	"sigmaos/hotel/proto"
	np "sigmaos/sigmap"
	"sigmaos/protdevsrv"
)

const (
	NUSER = 500
)

//type UserRequest struct {
//	Name     string
//	Password string
//}
//
//type UserResult struct {
//	OK string
//}

type User struct {
	Username string
	Password string
}

type Users struct {
	dbc *dbclnt.DbClnt
}

func RunUserSrv(n string) error {
	u := &Users{}
	pds, err := protdevsrv.MakeProtDevSrv(np.HOTELUSER, u)
	if err != nil {
		return err
	}
	dbc, err := dbclnt.MkDbClnt(pds.MemFs.FsLib(), np.DBD)
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
	p := ""
	for j := 0; j < 10; j++ {
		p += u
	}
	return p
}

func (s *Users) initDB() error {
	q := fmt.Sprintf("truncate user;")
	err := s.dbc.Exec(q)
	if err != nil {
		return err
	}
	for i := 0; i <= NUSER; i++ {
		suffix := strconv.Itoa(i)
		u := "Cornell_" + suffix
		p := MkPassword(suffix)
		sum := sha256.Sum256([]byte(p))
		q = fmt.Sprintf("INSERT INTO user (username, password) VALUES ('%v', '%x');", u, sum)
		err = s.dbc.Exec(q)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Users) CheckUser(req proto.UserRequest, res *proto.UserResult) error {
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
	sum := sha256.Sum256([]byte(req.Password))
	if fmt.Sprintf("%x", sum) != users[0].Password {
		return fmt.Errorf("Wrong password")
	}
	res.OK = "True"
	return nil
}

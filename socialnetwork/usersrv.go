package socialnetwork

import (
	"sigmaos/socialnetwork/proto"
	sp "sigmaos/sigmap"
	dbg "sigmaos/debug"
	"sigmaos/protdevsrv"
	"sigmaos/dbclnt"
	"sigmaos/fs"
	"crypto/sha256"
	"strconv"
	"fmt"
	"time"
)

const (
	USER_HB_FREQ = 1
	NUSER = 10
	USER_QUERY_OK = "OK"
)

type User struct {
	FirstName string
	LastName string
	Username string
	Password string
}

type UserSrv struct {
	dbc *dbclnt.DbClnt
	//TODO add cache client
}

func RunUserSrv(public bool) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "Creating user service\n")
	usrv := &UserSrv{}
	pds, err := protdevsrv.MakeProtDevSrvPublic(sp.SOCIAL_NETWORK_USER, usrv, public)
	if err != nil {
		return err
	}
	dbc, err := dbclnt.MkDbClnt(pds.MemFs.SigmaClnt().FsLib, sp.DBD)
	if err != nil {
		return err
	}
	usrv.dbc = dbc
	if err = usrv.initDB(); err != nil {
		return err
	}
	dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "Starting to run user service\n")
	go usrv.heartBeat(USER_HB_FREQ)
	return pds.RunServer()
}

func (usrv *UserSrv) CheckUser(ctx fs.CtxI, req proto.CheckUserRequest, res *proto.CheckUserResponse) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "Checking user %v\n", req)
	res.Ok = "No"
	exist, err := usrv.checkUserExist(req.Username)
	if  err != nil {
		return err
	} 
	if exist {
		res.Ok = USER_QUERY_OK
	}
	return nil
}

func (usrv *UserSrv) RegisterUser(ctx fs.CtxI, req proto.RegisterUserRequest, res *proto.RegisterUserResponse ) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "Register user %v\n", req)
	res.Ok = "No"
	exist, err := usrv.checkUserExist(req.Username)
	if  err != nil {
		return err
	} 
	if exist {
		res.Ok = fmt.Sprintf("Username %v already exist", req.Username)
		return nil
	}
	if err = usrv.createUser(req.Firstname, req.Lastname, req.Username, req.Password); err != nil {
		return err
	}	
	res.Ok = USER_QUERY_OK
	return nil
}

func (usrv *UserSrv) heartBeat(freq int) {
	for {
		time.Sleep(time.Duration(freq) * time.Second)
		dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "ALIVE!\n")
	}
}

func (usrv *UserSrv) initDB() error {
	q := fmt.Sprintf("truncate socialnetwork_user;")
	err := usrv.dbc.Exec(q)
	if err != nil {
		return err
	}
	for i := 0; i <= NUSER; i++ {
		suffix := strconv.Itoa(i)
		uname := "user_" + suffix
		fname := "FirstName" + suffix
		lname := "LastName" + suffix
		pswd := "p_" + uname
		if err = usrv.createUser(fname, lname, uname, pswd); err != nil {
			return err
		}		
	}
	return nil
}

func (usrv *UserSrv) checkUserExist(username string) (bool, error) {
	q := fmt.Sprintf("SELECT * from socialnetwork_user where username='%s';", username)
	var users []User
    if err := usrv.dbc.Query(q, &users); err != nil {
		return false, err
	}
	return len(users) != 0, nil
}

func (usrv *UserSrv) createUser(fname, lname, uname, pswd string) error {
	pswd_hashed := sha256.Sum256([]byte(pswd))
	q := fmt.Sprintf(
		"INSERT INTO socialnetwork_user (firstname, lastname, username, password)" + 
		" VALUES ('%v', '%v', '%v', '%x');", fname, lname, uname, pswd_hashed)
	return usrv.dbc.Exec(q)
}

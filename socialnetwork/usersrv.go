package socialnetwork

import (
	"sigmaos/socialnetwork/proto"
	sp "sigmaos/sigmap"
	dbg "sigmaos/debug"
	"sigmaos/protdevsrv"
	"sigmaos/dbclnt"
	"sigmaos/cacheclnt"
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
	Firstname string
	Lastname string
	Username string
	Password string
}

type UserSrv struct {
	dbc *dbclnt.DbClnt
	cachec *cacheclnt.CacheClnt
}

func RunUserSrv(public bool, jobname string) error {
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
	cachec, err := cacheclnt.MkCacheClnt(pds.MemFs.SigmaClnt().FsLib, jobname)
	if err != nil {
		return err
	}
	usrv.cachec = cachec
	dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "Initializing DB and starting user service\n")
	if err = usrv.initDB(); err != nil {
		return err
	}
	go usrv.heartBeat(USER_HB_FREQ)
	return pds.RunServer()
}

func (usrv *UserSrv) CheckUser(ctx fs.CtxI, req proto.CheckUserRequest, res *proto.UserResponse) error {
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

func (usrv *UserSrv) RegisterUser(ctx fs.CtxI, req proto.RegisterUserRequest, res *proto.UserResponse) error {
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

func (usrv *UserSrv) Login(ctx fs.CtxI, req proto.LoginRequest, res *proto.UserResponse) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "User login with %v\n", req)
	res.Ok = "Login Failure."
	user, err := usrv.getUserbyUname(req.Username)
	if  err != nil {
		return err
	}
	if user != nil && fmt.Sprintf("%x", sha256.Sum256([]byte(req.Password))) == user.Password {
		res.Ok = USER_QUERY_OK 			
	}
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
	for i := 0; i < NUSER; i++ {
		suffix := strconv.Itoa(i)
		uname := "user_" + suffix
		fname := "Firstname" + suffix
		lname := "Lastname" + suffix
		pswd := "p_" + uname
		if err = usrv.createUser(fname, lname, uname, pswd); err != nil {
			return err
		}		
	}
	return nil
}

func (usrv *UserSrv) checkUserExist(username string) (bool, error) {
	user, err := usrv.getUserbyUname(username)
	if err != nil {
		return false, err
	}
	return user != nil, nil
}

func (usrv *UserSrv) getUserbyUname(username string) (*User, error) {
	key := "user_by_uname_" + username
	user := &User{}
	if err := usrv.cachec.Get(key, user); err != nil {
		if !usrv.cachec.IsMiss(err) {
			return nil, err
		}
		dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "User cache miss: key %v\n", key)
		q := fmt.Sprintf("SELECT * from socialnetwork_user where username='%s';", username)
		var users []User
		if err := usrv.dbc.Query(q, &users); err != nil {
			return nil, err
		}
		if len(users) == 0 {
			return nil, nil
		}
		user = &users[0]
		usrv.cachec.Put(key, user)
	}
	dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "Found user for %v: %v\n", username, user)
	return user, nil
}

func (usrv *UserSrv) createUser(fname, lname, uname, pswd string) error {
	pswd_hashed := sha256.Sum256([]byte(pswd))
	q := fmt.Sprintf(
		"INSERT INTO socialnetwork_user (firstname, lastname, username, password)" + 
		" VALUES ('%v', '%v', '%v', '%x');", fname, lname, uname, pswd_hashed)
	return usrv.dbc.Exec(q)
}

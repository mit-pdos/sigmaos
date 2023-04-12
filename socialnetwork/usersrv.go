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
	"math/rand"
	"sync"
	"fmt"
	"time"
)

// YH:
// User service for social network
// for now we use sql instead of MongoDB

const (
	USER_HB_FREQ = 1
	USER_QUERY_OK = "OK"
)

type User struct {
	Userid int64 
	Firstname string
	Lastname string
	Username string
	Password string
}

type UserSrv struct {
	mu sync.Mutex
	dbc *dbclnt.DbClnt
	cachec *cacheclnt.CacheClnt
	sid int32  // sid is a random number between 0 and 2^30
	ucount int32 //This server may overflow with over 2^31 users
}

func RunUserSrv(public bool, jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "Creating user service\n")
	usrv := &UserSrv{}
	usrv.sid = rand.Int31n(536870912) // 2^29
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
	dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "Initializing DB and starting user service %v\n", usrv.sid)
	go usrv.heartBeat(USER_HB_FREQ)
	return pds.RunServer()
}

func (usrv *UserSrv) CheckUser(ctx fs.CtxI, req proto.CheckUserRequest, res *proto.UserResponse) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "Checking user at %v: %v\n", usrv.sid, req)
	res.Ok = "No"
	user, err := usrv.getUserbyUname(req.Username)
	if  err != nil {
		return err
	} 
	if user != nil {
		res.Userid = user.Userid
		res.Ok = USER_QUERY_OK
	}
	return nil
}

func (usrv *UserSrv) RegisterUser(ctx fs.CtxI, req proto.RegisterUserRequest, res *proto.UserResponse) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "Register user at %v: %v\n", usrv.sid, req)
	res.Ok = "No"
	user, err := usrv.getUserbyUname(req.Username)
	if  err != nil {
		return err
	} 
	if user != nil {
		res.Ok = fmt.Sprintf("Username %v already exist", req.Username)
		return nil
	}
	pswd_hashed := sha256.Sum256([]byte(req.Password))
	userid := usrv.getNextUserId()
	q := fmt.Sprintf(
		"INSERT INTO socialnetwork_user (firstname, lastname, username, password, userid)" +
		" VALUES ('%v', '%v', '%v', '%x', '%v');", 
		req.Firstname, req.Lastname, req.Username, pswd_hashed, userid)
	if qErr := usrv.dbc.Exec(q); qErr != nil {
		return qErr
	}
	res.Ok = USER_QUERY_OK
	res.Userid = userid
	return nil
}

func (usrv *UserSrv) incCountSafe() int32 {
	usrv.mu.Lock()
	defer usrv.mu.Unlock()
	usrv.ucount++
	return usrv.ucount
}

func (usrv *UserSrv) getNextUserId() int64 {
	return int64(usrv.sid) * 1E10 + int64(usrv.incCountSafe())
}

func (usrv *UserSrv) Login(ctx fs.CtxI, req proto.LoginRequest, res *proto.UserResponse) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "User login with %v: %v\n", usrv.sid, req)
	res.Ok = "Login Failure."
	user, err := usrv.getUserbyUname(req.Username)
	if  err != nil {
		return err
	}
	if user != nil && fmt.Sprintf("%x", sha256.Sum256([]byte(req.Password))) == user.Password {
		res.Ok = USER_QUERY_OK 			
		res.Userid = user.Userid

	}
	return nil	
}

func (usrv *UserSrv) heartBeat(freq int) {
	for {
		time.Sleep(time.Duration(freq) * time.Second)
		dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "ALIVE!\n")
	}
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

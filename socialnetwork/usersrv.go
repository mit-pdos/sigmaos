package socialnetwork

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
	"sigmaos/cacheclnt"
	"sigmaos/dbclnt"
	dbg "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/protdevsrv"
	sp "sigmaos/sigmap"
	"sigmaos/socialnetwork/proto"
	"sync"
)

// YH:
// User service for social network
// for now we use sql instead of MongoDB

const (
	USER_QUERY_OK = "OK"
	USER_CACHE_PREFIX = "user_"
)

type UserSrv struct {
	mu     sync.Mutex
	dbc    *dbclnt.DbClnt
	cachec *cacheclnt.CacheClnt
	sid    int32 // sid is a random number between 0 and 2^30
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
	fsls := MakeFsLibs(sp.SOCIAL_NETWORK_USER)
	cachec, err := cacheclnt.MkCacheClnt(fsls, jobname)
	if err != nil {
		return err
	}
	usrv.cachec = cachec
	dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "Starting user service %v\n", usrv.sid)
	return pds.RunServer()
}

func (usrv *UserSrv) CheckUser(ctx fs.CtxI, req proto.CheckUserRequest, res *proto.CheckUserResponse) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "Checking user at %v: %v\n", usrv.sid, req.Usernames)
	userids := make([]int64, len(req.Usernames))
	res.Ok = "No"
	missing := false
	for idx, username := range req.Usernames {
		user, err := usrv.getUserbyUname(username)
		if err != nil {
			return err
		}
		if user == nil {
			userids[idx] = int64(-1)
			missing = true
		} else {
			userids[idx] = user.Userid
		}
	}
	res.Userids = userids
	if !missing {
		res.Ok = USER_QUERY_OK
	}
	return nil
}

func (usrv *UserSrv) RegisterUser(ctx fs.CtxI, req proto.RegisterUserRequest, res *proto.UserResponse) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "Register user at %v: %v\n", usrv.sid, req)
	res.Ok = "No"
	user, err := usrv.getUserbyUname(req.Username)
	if err != nil {
		return err
	}
	if user != nil {
		res.Ok = fmt.Sprintf("Username %v already exist", req.Username)
		return nil
	}
	pswd_hashed := sha256.Sum256([]byte(req.Password))
	userid := usrv.getNextUserId()
	q := fmt.Sprintf(
		"INSERT INTO socialnetwork_user (firstname, lastname, username, password, userid)"+
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
	return int64(usrv.sid)*1e10 + int64(usrv.incCountSafe())
}

func (usrv *UserSrv) Login(ctx fs.CtxI, req proto.LoginRequest, res *proto.UserResponse) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "User login with %v: %v\n", usrv.sid, req)
	res.Ok = "Login Failure."
	user, err := usrv.getUserbyUname(req.Username)
	if err != nil {
		return err
	}
	if user != nil && fmt.Sprintf("%x", sha256.Sum256([]byte(req.Password))) == user.Password {
		res.Ok = USER_QUERY_OK
		res.Userid = user.Userid
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

func (usrv *UserSrv) getUserbyUname(username string) (*proto.User, error) {
	key := USER_CACHE_PREFIX + username
	user := &proto.User{}
	if err := usrv.cachec.Get(key, user); err != nil {
		if !usrv.cachec.IsMiss(err) {
			return nil, err
		}
		dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "User %v cache miss\n", key)
		q := fmt.Sprintf("SELECT * from socialnetwork_user where username='%s';", username)
		var users []proto.User
		if err := usrv.dbc.Query(q, &users); err != nil {
			return nil, err
		}
		if len(users) == 0 {
			return nil, nil
		}
		user = &users[0]
		dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "Found user %v in DB: %v\n", username, user)
		usrv.cachec.Put(key, user)
	} else {
		dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "Found user %v in cache!\n", username)
	}
	return user, nil
}

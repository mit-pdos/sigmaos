package socialnetwork

import (
	sp "sigmaos/sigmap"
	"sigmaos/sigmaclnt"
	"sigmaos/dbclnt"
	"crypto/sha256"
	"strconv"
	"fmt"
)

// YH:
// Utility class to populate initial DB contents.

const (
	NUSER = 10
)

type DBUtil struct {
	dbc *dbclnt.DbClnt
}

func MakeDBUtil(sc *sigmaclnt.SigmaClnt) (*DBUtil, error) {
	dbc, err := dbclnt.MkDbClnt(sc.FsLib, sp.DBD)
	if err != nil {
		return nil, err
	}
	return &DBUtil{dbc}, nil
}

func (dbu *DBUtil) GetDbc() *dbclnt.DbClnt {
	return dbu.dbc
}

func (dbu *DBUtil) Clear() error {
	if err := dbu.dbc.Exec("truncate socialnetwork_user;"); err != nil {
		return err
	}
	if err := dbu.dbc.Exec("truncate socialnetwork_graph;"); err != nil {
		return err
	}
	if err := dbu.dbc.Exec("truncate socialnetwork_post;"); err != nil {
		return err
	}
	if err := dbu.dbc.Exec("truncate socialnetwork_timeline;"); err != nil {
		return err
	}
	if err := dbu.dbc.Exec("truncate socialnetwork_url;"); err != nil {
		return err
	}
	return nil
}

func (dbu *DBUtil) InitUser() error {
	// create NUSER test users
	for i := 0; i < NUSER; i++ {
		suffix := strconv.Itoa(i)
		uname := "user_" + suffix
		fname := "Firstname" + suffix
		lname := "Lastname" + suffix
		pswd := "p_" + uname
		pswd_hashed := sha256.Sum256([]byte(pswd))
		userid := i
		q := fmt.Sprintf(
			"INSERT INTO socialnetwork_user (firstname, lastname, username, password, userid)" +
			" VALUES ('%v', '%v', '%v', '%x', '%v');", fname, lname, uname, pswd_hashed, userid)
		if err := dbu.dbc.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (dbu *DBUtil) InitGraph() error {
	//user i follows user i+1
	for i := 0; i < NUSER-1; i++ {
		q := fmt.Sprintf(
			"INSERT INTO socialnetwork_graph (followerid, followeeid) VALUES ('%v', '%v');", i, i+1)
		if err := dbu.dbc.Exec(q); err != nil {
			return err
		}
	}
	return nil
}


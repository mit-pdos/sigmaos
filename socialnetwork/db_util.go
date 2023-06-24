package socialnetwork

import (
	"sigmaos/sigmaclnt"
	"sigmaos/mongoclnt"
	"crypto/sha256"
	"strconv"
	"fmt"
	"gopkg.in/mgo.v2/bson"
)

// YH:
// Utility class to populate initial DB contents.

const (
	NUSER           = 10
	SN_DB           = "socialnetwork"
	USER_COL        = "user"
	GRAPH_FLWER_COL = "graph-follower"
	GRAPH_FLWEE_COL = "graph-followee"
	POST_COL        = "post"
	URL_COL         = "url"
	TIMELINE_COL    = "timeline"
	MEDIA_COL       = "media"
)

type DBUtil struct {
	mongoc *mongoclnt.MongoClnt
}

func MakeDBUtil(sc *sigmaclnt.SigmaClnt) (*DBUtil, error) {
	mongoc, err := mongoclnt.MkMongoClnt(sc.FsLib)
	if err != nil {
		return nil, err
	}
	return &DBUtil{mongoc}, nil
}

func (dbu *DBUtil) Clear() error {
	dbu.mongoc.DropCollection(SN_DB, USER_COL)
	dbu.mongoc.DropCollection(SN_DB, GRAPH_FLWER_COL)
	dbu.mongoc.DropCollection(SN_DB, GRAPH_FLWEE_COL)
	dbu.mongoc.DropCollection(SN_DB, POST_COL)
	dbu.mongoc.DropCollection(SN_DB, TIMELINE_COL)
	dbu.mongoc.DropCollection(SN_DB, URL_COL)
	dbu.mongoc.DropCollection(SN_DB, MEDIA_COL)
	return nil
}

func (dbu *DBUtil) InitUser() error {
	// create NUSER test users
	dbu.mongoc.EnsureIndex(SN_DB, USER_COL, []string{"userid"})
	for i := 0; i < NUSER; i++ {
		suffix := strconv.Itoa(i)
		newUser := User{
			Userid: int64(i),
			Username: "user_" + suffix,
			Lastname: "Lastname" + suffix,
			Firstname: "Firstname" + suffix,
			Password: fmt.Sprintf("%x", sha256.Sum256([]byte("p_user_" + suffix)))}
		if err := dbu.mongoc.Insert(SN_DB, USER_COL, newUser); err != nil {
			return err
		}
	}
	return nil
}

func (dbu *DBUtil) InitGraph() error {
	//user i follows user i+1
	dbu.mongoc.EnsureIndex(SN_DB, GRAPH_FLWER_COL, []string{"userid"})
	dbu.mongoc.EnsureIndex(SN_DB, GRAPH_FLWEE_COL, []string{"userid"})
	for i := 0; i < NUSER-1; i++ {
		err1 := dbu.mongoc.Upsert(SN_DB, GRAPH_FLWER_COL,
			bson.M{"userid": int64(i+1)}, bson.M{"$addToSet": bson.M{"edges": int64(i)}})
		err2 := dbu.mongoc.Upsert(SN_DB, GRAPH_FLWEE_COL,
			bson.M{"userid": int64(i)}, bson.M{"$addToSet": bson.M{"edges": int64(i+1)}})
		if err1 != nil || err2 != nil {
			err := fmt.Errorf("error updating graph %v %v", err1, err2)
			return err
		}
	}
	return nil
}


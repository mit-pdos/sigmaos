package socialnetwork

import (
	"sigmaos/sigmaclnt"
	"sigmaos/mongoclnt"
	dbg "sigmaos/debug"
	"crypto/sha256"
	"strconv"
	"time"
	"fmt"
	"math"
	"sync"
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
	dbu.mongoc.RemoveAll(SN_DB, USER_COL)
	dbu.mongoc.RemoveAll(SN_DB, GRAPH_FLWER_COL)
	dbu.mongoc.RemoveAll(SN_DB, GRAPH_FLWEE_COL)
	dbu.mongoc.RemoveAll(SN_DB, POST_COL)
	dbu.mongoc.RemoveAll(SN_DB, TIMELINE_COL)
	dbu.mongoc.RemoveAll(SN_DB, URL_COL)
	dbu.mongoc.RemoveAll(SN_DB, MEDIA_COL)
	return nil
}

func (dbu *DBUtil) InitUser() error {
	// create NUSER test users
	dbu.mongoc.EnsureIndex(SN_DB, USER_COL, []string{"username"})
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


type Counter struct {
	mu    sync.Mutex
	count int64
	sum    int64
	ssum  int64
	max   int64
	min   int64
	label string
}

func MakeCounter(str string) *Counter {
	return &Counter{label: str, count: 0, sum: 0, ssum: 0, min: 10000000, max: 0}
}

func (c *Counter) AddOne(val int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count += 1
	c.sum += val
	c.ssum += val*val
	if val > c.max {
		c.max = val
	}
	if val < c.min {
		c.min = val
	}
	if c.count == 1000 {
		avg := c.sum / c.count
		std := math.Sqrt(float64(c.ssum/c.count - avg*avg))
		dbg.DPrintf(dbg.ALWAYS, 
			"Stats for %v: max = %v; min=%v, avg=%v, std=%v\n", c.label, c.max, c.min, avg, std)
		c.count = 0
		c.sum = 0
		c.ssum = 0
		c.max = 0
		c.min = 10000000
	}
}

func (c* Counter) AddTimeSince(t0 time.Time) {
	c.AddOne(time.Since(t0).Microseconds())
}

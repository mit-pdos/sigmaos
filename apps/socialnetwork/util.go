package socialnetwork

import (
	"crypto/sha256"
	"fmt"
	"github.com/montanaflynn/stats"
	"gopkg.in/mgo.v2/bson"
	"math/rand"
	dbg "sigmaos/debug"
	mongoclnt "sigmaos/proxy/mongo/clnt"
	"sigmaos/sigmaclnt"
	"strconv"
	"sync"
	"time"
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
	N_COUNT         = 5000
)

var letterRunes = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randString(n, l int, r *rand.Rand) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[r.Intn(l)]
	}
	return string(b)
}
func RandString(n int, r *rand.Rand) string {
	return randString(n, len(letterRunes), r)
}

func RandNumberString(n int, r *rand.Rand) string {
	return randString(n, 10, r)
}

type DBUtil struct {
	mongoc *mongoclnt.MongoClnt
}

func NewDBUtil(sc *sigmaclnt.SigmaClnt) (*DBUtil, error) {
	mongoc, err := mongoclnt.NewMongoClnt(sc.FsLib)
	if err != nil {
		return nil, err
	}
	return &DBUtil{mongoc}, nil
}

func (dbu *DBUtil) GetURL() (string, error) {
	return dbu.mongoc.GetMongoURL()
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
			Userid:    int64(i),
			Username:  "user_" + suffix,
			Lastname:  "Lastname" + suffix,
			Firstname: "Firstname" + suffix,
			Password:  fmt.Sprintf("%x", sha256.Sum256([]byte("p_user_"+suffix)))}
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
			bson.M{"userid": int64(i + 1)}, bson.M{"$addToSet": bson.M{"edges": int64(i)}})
		err2 := dbu.mongoc.Upsert(SN_DB, GRAPH_FLWEE_COL,
			bson.M{"userid": int64(i)}, bson.M{"$addToSet": bson.M{"edges": int64(i + 1)}})
		if err1 != nil || err2 != nil {
			err := fmt.Errorf("error updating graph %v %v", err1, err2)
			return err
		}
	}
	return nil
}

type Counter struct {
	mu    sync.Mutex
	label string
	count int64
	vals  []int64
}

func NewCounter(str string) *Counter {
	return &Counter{label: str, count: 0, vals: make([]int64, N_COUNT)}
}

func (c *Counter) AddOne(val int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.vals[c.count] = val
	c.count += 1
	if c.count == N_COUNT {
		c.printStats()
		c.count = 0
		c.vals = make([]int64, N_COUNT)
	}
}

func (c *Counter) AddTimeSince(t0 time.Time) {
	c.AddOne(time.Since(t0).Microseconds())
}

func (c *Counter) printStats() {
	fVals := make([]float64, N_COUNT)
	for i := range c.vals {
		fVals[i] = float64(c.vals[i])
	}
	mean, _ := stats.Mean(fVals)
	std, _ := stats.StandardDeviation(fVals)
	lat50P, _ := stats.Percentile(fVals, 50)
	lat75P, _ := stats.Percentile(fVals, 75)
	lat99P, _ := stats.Percentile(fVals, 99)
	min, _ := stats.Min(fVals)
	max, _ := stats.Max(fVals)
	dbg.DPrintf(dbg.ALWAYS,
		"Stats for %v: mean=%v; std=%v; min=%v; max=%v; 50P,75P,99P=%v,%v,%v;",
		c.label, mean, std, min, max, lat50P, lat75P, lat99P)
}

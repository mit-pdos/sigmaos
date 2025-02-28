package benchmarks_test

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"math/rand"
	"net"
	"os"
	"os/exec"
	sn "sigmaos/apps/socialnetwork"
	"sigmaos/benchmarks/loadgen"
	dbg "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	"sigmaos/util/perf"
	rd "sigmaos/util/rand"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	K8_FWD_PORT    = "9090"
	N_BENCH_USER   = 962 // from "data/socfb-Reed98/socfb-Reed98.nodes"
	HOME_RATIO     = 0.6
	TIMELINE_RATIO = 0.3
	SN_RAND_SEED   = 54321
)

var MONGO_URL string

func getDefaultSrvs() []sn.Srv {
	return []sn.Srv{
		sn.Srv{"socialnetwork-user", nil, 2000},
		sn.Srv{"socialnetwork-graph", nil, 2000},
		sn.Srv{"socialnetwork-post", nil, 2000},
		sn.Srv{"socialnetwork-timeline", nil, 2000},
		sn.Srv{"socialnetwork-home", nil, 2000},
		sn.Srv{"socialnetwork-url", nil, 2000},
		sn.Srv{"socialnetwork-text", nil, 2000},
		sn.Srv{"socialnetwork-compose", nil, 2000},
		sn.Srv{"socialnetwork-frontend", nil, 2000}}
}

func init() {
	flag.StringVar(&MONGO_URL, "mongourl", "10.10.1.1:4407", "Addr of mongo server.")
}

type snFn func(wc *sn.WebClnt, r *rand.Rand)

type SocialNetworkJobInstance struct {
	sigmaos    bool
	readonly   bool
	k8ssrvaddr string
	job        string
	dur        []time.Duration
	maxrps     []int
	ncache     int
	ready      chan bool
	snCfg      *sn.SocialNetworkConfig
	lgs        []*loadgen.LoadGenerator
	p          *perf.Perf
	wc         *sn.WebClnt
	*test.RealmTstate
}

func NewSocialNetworkJob(
	ts *test.RealmTstate, p *perf.Perf, sigmaos, readonly bool,
	durStr, maxrpsStr string, ncache int) *SocialNetworkJobInstance {
	ji := &SocialNetworkJobInstance{}
	ji.sigmaos = sigmaos
	ji.job = rd.String(8)
	ji.ready = make(chan bool)
	ji.RealmTstate = ts
	ji.p = p
	ji.ncache = ncache
	ji.readonly = readonly
	// parse duration and rpss
	durs := strings.Split(durStr, ",")
	maxrpss := strings.Split(maxrpsStr, ",")
	assert.Equal(ts.Ts.T, len(durs), len(maxrpss), "Non-matching lengths: %v %v", durStr, maxrpsStr)
	ji.dur = make([]time.Duration, 0, len(durs))
	ji.maxrps = make([]int, 0, len(durs))
	for i := range durs {
		d, err := time.ParseDuration(durs[i])
		assert.Nil(ts.Ts.T, err, "Bad duration %v", err)
		n, err := strconv.Atoi(maxrpss[i])
		assert.Nil(ts.Ts.T, err, "Bad duration %v", err)
		ji.dur = append(ji.dur, d)
		ji.maxrps = append(ji.maxrps, n)
	}
	// populate DB
	// start social network
	var err error
	if sigmaos {
		initUserAndGraph(ts.Ts.T, MONGO_URL)
		ji.snCfg, err = sn.NewConfig(
			ts.SigmaClnt, ji.job, getDefaultSrvs(), ncache, true)
		assert.Nil(ts.Ts.T, err, "Error Make social network job: %v", err)
	} else {
		ji.snCfg, err = sn.NewConfig(ts.SigmaClnt, ji.job, nil, 0, false)
		p := sn.JobHTTPAddrsPath(ji.job)
		h, po, err := net.SplitHostPort(K8S_ADDR)
		assert.Nil(ts.Ts.T, err, "Err split host port %v: %v", ji.k8ssrvaddr, err)
		port, err := strconv.Atoi(po)
		assert.Nil(ts.Ts.T, err, "Err parse port %v: %v", po, err)
		addr := sp.NewTaddr(sp.Tip(h), sp.INNER_CONTAINER_IP, sp.Tport(port))
		mnt := sp.NewEndpoint(sp.EXTERNAL_EP, []*sp.Taddr{addr})
		assert.Nil(ts.Ts.T, ts.MkEndpointFile(p, mnt))
		// forward mongo port and init users and graphs.
		cmd := exec.Command("kubectl", "port-forward", "svc/mongodb-sn", K8_FWD_PORT+":27017")
		assert.Nil(ts.Ts.T, cmd.Start())
		defer cmd.Process.Kill()
		initUserAndGraph(ts.Ts.T, "localhost:"+K8_FWD_PORT)
	}
	ji.wc = sn.NewWebClnt(ts.FsLib, ji.job)
	// Make a load generators.
	ji.lgs = make([]*loadgen.LoadGenerator, 0, len(ji.dur))
	for i := range ji.dur {
		ji.lgs = append(
			ji.lgs, loadgen.NewLoadGenerator(ji.dur[i], ji.maxrps[i],
				func(r *rand.Rand) (time.Duration, bool) {
					randOps(ts.Ts.T, ji.wc, r, ji.readonly)
					return 0, false
				}))
	}
	// warmup with writes for read-only runs
	if ji.readonly {
		dbg.DPrintf(dbg.TEST, "Warming up with writes for read-only test")
		var wg sync.WaitGroup
		for i := 0; i < N_BENCH_USER; i++ {
			wg.Add(1)
			go func(wg *sync.WaitGroup, t *testing.T, wc *sn.WebClnt, i int) {
				defer wg.Done()
				r := rand.New(rand.NewSource(SN_RAND_SEED + int64(i+1)))
				randCompose(t, wc, r)
				randCompose(t, wc, r)
			}(&wg, ts.Ts.T, ji.wc, i)
		}
		wg.Wait()
		dbg.DPrintf(dbg.TEST, "Done warming up")
	}
	return ji
}

func (ji *SocialNetworkJobInstance) StartSocialNetworkJob() {
	dbg.DPrintf(dbg.TEST, "StartSocialNetworkJob dur %v ncache %v maxrps %v kubernetes (%v,%v)", ji.dur, ji.ncache, ji.maxrps, !ji.sigmaos, ji.k8ssrvaddr)
	var wg sync.WaitGroup
	for _, lg := range ji.lgs {
		wg.Add(1)
		go func(lg *loadgen.LoadGenerator, wg *sync.WaitGroup) {
			defer wg.Done()
			lg.Calibrate()
		}(lg, &wg)
	}
	wg.Wait()
	_, err := ji.wc.StartRecording()
	if err != nil {
		dbg.DFatalf("Can't start recording: %v", err)
	}
	randReadTimeline(ji.RealmTstate.Ts.T, ji.wc, rand.New(rand.NewSource(999)))
	for i, lg := range ji.lgs {
		dbg.DPrintf(dbg.TEST, "Run load generator rps %v dur %v", ji.maxrps[i], ji.dur[i])
		lg.Run()
	}
	dbg.DPrintf(dbg.TEST, "Done running SocialNetworkJob")
}

func (ji *SocialNetworkJobInstance) Wait() {
	dbg.DPrintf(dbg.TEST, "extra sleep")
	time.Sleep(1 * time.Second)
	if ji.p != nil {
		ji.p.Done()
	}
	dbg.DPrintf(dbg.TEST, "Evicting social network procs")
	if ji.sigmaos {
		err := ji.snCfg.Stop()
		assert.Nil(ji.Ts.T, err, "stop %v", err)
	}
	dbg.DPrintf(dbg.TEST, "Done evicting social network procs")
	for _, lg := range ji.lgs {
		dbg.DPrintf(dbg.TEST, "Data:\n%v", lg.StatsDataString())
	}
	for _, lg := range ji.lgs {
		lg.Stats()
	}
}

func (ji *SocialNetworkJobInstance) requestK8sStats() {
	rep, err := ji.wc.SaveResults()
	assert.Nil(ji.Ts.T, err, "Save results: %v", err)
	assert.Equal(ji.Ts.T, rep, "Done!", "Save results not ok: %v", rep)
}

func initUserAndGraph(t *testing.T, mongoUrl string) {
	session, err := mgo.Dial(mongoUrl)
	assert.Nil(t, err, "Cannot connect to Mongo (%v): %v", mongoUrl, err)
	// clear all tables
	session.DB(sn.SN_DB).C(sn.USER_COL).RemoveAll(&bson.M{})
	session.DB(sn.SN_DB).C(sn.GRAPH_FLWER_COL).RemoveAll(&bson.M{})
	session.DB(sn.SN_DB).C(sn.GRAPH_FLWEE_COL).RemoveAll(&bson.M{})
	session.DB(sn.SN_DB).C(sn.POST_COL).RemoveAll(&bson.M{})
	session.DB(sn.SN_DB).C(sn.TIMELINE_COL).RemoveAll(&bson.M{})
	session.DB(sn.SN_DB).C(sn.URL_COL).RemoveAll(&bson.M{})
	session.DB(sn.SN_DB).C(sn.MEDIA_COL).RemoveAll(&bson.M{})
	// insert users
	session.DB(sn.SN_DB).C(sn.USER_COL).EnsureIndexKey("username")
	dbg.DPrintf(dbg.TEST, "Inserting users")
	for i := 0; i < N_BENCH_USER; i++ {
		suffix := strconv.Itoa(i)
		newUser := sn.User{
			Userid:    int64(i),
			Username:  "user_" + suffix,
			Lastname:  "Lastname" + suffix,
			Firstname: "Firstname" + suffix,
			Password:  fmt.Sprintf("%x", sha256.Sum256([]byte("p_user_"+suffix)))}
		err := session.DB(sn.SN_DB).C(sn.USER_COL).Insert(newUser)
		assert.Nil(t, err, "cannot insert user: %v", err)
	}
	// insert graphs
	b, err := os.ReadFile("../socialnetwork/data/socfb-Reed98/socfb-Reed98.edges")
	assert.Nil(t, err, "Cannot open edge file: %v", err)
	dbg.DPrintf(dbg.TEST, "Inserting graphs")
	for _, line := range strings.FieldsFunc(string(b), func(c rune) bool { return c == '\n' }) {
		ids := strings.Split(line, " ")
		followerId, _ := strconv.ParseInt(ids[0], 10, 64)
		followeeId, _ := strconv.ParseInt(ids[1], 10, 64)
		_, err1 := session.DB(sn.SN_DB).C(sn.GRAPH_FLWER_COL).Upsert(
			bson.M{"userid": followeeId}, bson.M{"$addToSet": bson.M{"edges": followerId}})
		_, err2 := session.DB(sn.SN_DB).C(sn.GRAPH_FLWEE_COL).Upsert(
			bson.M{"userid": followerId}, bson.M{"$addToSet": bson.M{"edges": followeeId}})
		assert.True(t, err1 == nil && err2 == nil, "cannot insert graph: %v; %v", err1, err2)
	}
	dbg.DPrintf(dbg.TEST, "Complete mongo inserts!")
	var results []sn.EdgeInfo
	session.DB(sn.SN_DB).C(sn.GRAPH_FLWER_COL).Find(bson.M{"userid": int64(0)}).All(&results)
	assert.Equal(t, 73, len(results[0].Edges))
}

func randCompose(t *testing.T, wc *sn.WebClnt, r *rand.Rand) {
	uIdStr := strconv.Itoa(r.Intn(N_BENCH_USER))
	uname := "user_" + uIdStr
	nMentions := r.Intn(5)
	nUrls := r.Intn(5)
	nMedias := r.Intn(4)
	text := sn.RandString(256, r)
	for i := 0; i < nMentions; i++ {
		text += " @" + "user_" + strconv.Itoa(r.Intn(N_BENCH_USER))
	}
	for i := 0; i < nUrls; i++ {
		text += " http://" + sn.RandString(64, r)
	}
	strs := make([]string, 0)
	for i := 0; i < nMedias; i++ {
		strs = append(strs, sn.RandNumberString(18, r))
	}
	s, err := wc.ComposePost(uname, uIdStr, text, "post", strings.Join(strs, ","))
	assert.Nil(t, err)
	assert.Equal(t, "Compose successfully!", s)
}

func randReadHome(t *testing.T, wc *sn.WebClnt, r *rand.Rand) {
	uIdStr := strconv.Itoa(r.Intn(N_BENCH_USER))
	_, err := wc.ReadHome(uIdStr, "10")
	assert.Nil(t, err, "Cannot read home timeline: %v", err)
}

func randReadTimeline(t *testing.T, wc *sn.WebClnt, r *rand.Rand) {
	uIdStr := strconv.Itoa(r.Intn(N_BENCH_USER))
	_, err := wc.ReadTimeline(uIdStr, "10")
	assert.Nil(t, err, "Cannot read user timeline: %v", err)
}

func randOps(t *testing.T, wc *sn.WebClnt, r *rand.Rand, readonly bool) {
	ratio := float64(r.Intn(10000)) / 10000
	if ratio < TIMELINE_RATIO {
		randReadTimeline(t, wc, r)
	} else if ratio < TIMELINE_RATIO+HOME_RATIO || readonly {
		randReadHome(t, wc, r)
	} else {
		randCompose(t, wc, r)
	}
}

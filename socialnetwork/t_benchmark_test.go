package socialnetwork_test

import (
	"testing"
	"sigmaos/test"
	dbg "sigmaos/debug"
	sn "sigmaos/socialnetwork"
	sp "sigmaos/sigmap"
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"strconv"
	"crypto/sha256"
	"flag"
	"fmt"
	"os/exec"
	"os"
	"strings"
	"time"
	"math/rand"
	"sigmaos/loadgen"
)

const (
	K8S_MONGO_FWD_PORT = "9090"
	N_BENCH_USER       = 962 // from "data/socfb-Reed98/socfb-Reed98.nodes"
	COMPOSE_RATIO      = 0.1
	HOME_RATIO         = 0.6
	TIMELINE_RATIO     = 0.3
	LOAD_DUR           = 10
	LOAD_MAX_RPS       = 2000
)

var K8S_ADDR string
var MONGO_URL string
var BENCH_TEST bool

func init() {
	flag.StringVar(&K8S_ADDR, "k8saddr", "", "Addr of k8s frontend.")
	flag.StringVar(&MONGO_URL, "mongourl", "172.17.0.3:27017", "Addr of mongo server.")
	flag.BoolVar(&BENCH_TEST, "benchtest", false, "Is this a benchmark test?")
}

func initUserAndGraph(t *testing.T, mongoUrl string) {
	session, err := mgo.Dial(mongoUrl)
	assert.Nil(t, err, "Cannot connect to Mongo: %v", err)
	// insert users
	session.DB(sn.SN_DB).C(sn.USER_COL).EnsureIndexKey("username")
	dbg.DPrintf(dbg.TEST, "Inserting users")
	for i := 0; i < N_BENCH_USER; i++ {
		suffix := strconv.Itoa(i)
		newUser := sn.User{
			Userid: int64(i),
			Username: "user_" + suffix,
			Lastname: "Lastname" + suffix,
			Firstname: "Firstname" + suffix,
			Password: fmt.Sprintf("%x", sha256.Sum256([]byte("p_user_" + suffix)))}

		err := session.DB(sn.SN_DB).C(sn.USER_COL).Insert(newUser)
		assert.Nil(t, err, "cannot insert user: %v", err)
	}
	// insert graphs
	b, err := os.ReadFile("data/socfb-Reed98/socfb-Reed98.edges")
	assert.Nil(t, err, "Cannot open edge file: %v", err)
	dbg.DPrintf(dbg.TEST, "Inserting graphs")
	for _, line := range strings.FieldsFunc(string(b), func(c rune) bool {return c =='\n'}) {
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

func setupSigmaState(t *testing.T) *TstateSN {
	tssn := makeTstateSN(t, []sn.Srv{
		sn.Srv{"socialnetwork-user", test.Overlays, 1000},
		sn.Srv{"socialnetwork-graph", test.Overlays, 1000},
		sn.Srv{"socialnetwork-post", test.Overlays, 1000},
		sn.Srv{"socialnetwork-timeline", test.Overlays, 1000},
		sn.Srv{"socialnetwork-home", test.Overlays, 1000},
		sn.Srv{"socialnetwork-url", test.Overlays, 1000},
		sn.Srv{"socialnetwork-text", test.Overlays, 1000},
		sn.Srv{"socialnetwork-compose", test.Overlays, 1000},
		sn.Srv{"socialnetwork-frontend", test.Overlays, 1000}}, NSHARD)
	initUserAndGraph(t, MONGO_URL)
	return tssn
}


func setupK8sState(t *testing.T) *TstateSN {
	// Advertise server address
	tssn := makeTstateSN(t, nil, 0)
	p := sn.JobHTTPAddrsPath(tssn.jobname)
	mnt := sp.MkMountService(sp.MkTaddrs([]string{K8S_ADDR}))
	assert.Nil(t, tssn.MountService(p, mnt))
	// forward mongo port and init users and graphs.
	cmd := exec.Command("kubectl", "port-forward", "svc/mongodb-sn", K8S_MONGO_FWD_PORT+":27017")
	assert.Nil(t, cmd.Start())
	defer cmd.Process.Kill()
	initUserAndGraph(t, "localhost:"+K8S_MONGO_FWD_PORT)
	return tssn
}

func testTemplate(t *testing.T, isBenchTest bool, testFunc func(*testing.T, *sn.WebClnt)) {
	if isBenchTest && !BENCH_TEST {
		dbg.DPrintf(dbg.ALWAYS, "Skipping benchmark test")
		return
	}
	var tssn *TstateSN
	if K8S_ADDR == "" {
		dbg.DPrintf(dbg.ALWAYS, "No k8s addr. Running SigmaOS")
		tssn = setupSigmaState(t)
	} else {
		dbg.DPrintf(dbg.ALWAYS, "Running K8s at %v", K8S_ADDR)
		tssn = setupK8sState(t)
	}
	wc := sn.MakeWebClnt(tssn.FsLib, tssn.jobname)

	// run tests
	testFunc(t, wc)

	//stop server
	assert.Nil(t, tssn.Shutdown())
}

func TestBenchmarkSeqCompose(t *testing.T) {
	testTemplate(t, true, testSeqComposeInner)
}

func TestBenchmarkSeqMix(t *testing.T) {
	testTemplate(t, true, testSeqMixInner)
}

func TestFrontend(t *testing.T) {
	testTemplate(t, false, testFrontendInner)
}

func TestLoadgen(t *testing.T) {
	testTemplate(t, true, testLoadgenInner)
}

// Definition of benchmark functions
func testSeqComposeInner(t *testing.T, wc *sn.WebClnt) {
	rmsg, err := wc.StartRecording()
	assert.Nil(t, err)
	assert.Equal(t, "Started recording!", rmsg)
	dbg.DPrintf(dbg.TEST, "Start time : %v", time.Now().String())
	t0 := time.Now()

	// log in
	users := make([]string, N_BENCH_USER)
	for i := 0; i < N_BENCH_USER; i++ {
		users[i] = strconv.Itoa(i)
		s, err := wc.Login("user_" + users[i], "p_user_" + users[i])
		assert.Nil(t, err)
		assert.Equal(t, "Login successfully!", s)
	}

	// compose posts and check timelines. check 5 times for each compose
	N_COMPOSE := 250
	for i := 0; i < N_COMPOSE; i++ {
		if i % (N_COMPOSE/10) == 0 {
			dbg.DPrintf(dbg.TEST, "Check point at %v: %v", i, time.Since(t0).Microseconds())
		}
		meStr := users[i%N_BENCH_USER]
		msg := fmt.Sprintf(
			"My post #%d! @user_%d@user_%d https://www.google.com/?search=%d",
			i, (i+2)%N_BENCH_USER, (i+7)%N_BENCH_USER, i)
		s, err := wc.ComposePost("user_"+meStr, "", msg, "post", "")
		assert.Nil(t, err)
		assert.Equal(t, "Compose successfully!", s)
	}
	dbg.DPrintf(dbg.TEST, "Finish time : %v", time.Since(t0).Microseconds())
}

func testFrontendInner(t *testing.T, wc *sn.WebClnt) {
	// log in
	s, err := wc.Login("user_0", "p_user_0")
	assert.Nil(t, err)
	assert.Equal(t, "Login successfully!", s)

	s, err = wc.Login("user_1", "p_user_0")
	assert.Nil(t, err)
	assert.Equal(t, "Failed. Please check your username and password. ", s)

	// compose posts
	s, err = wc.ComposePost(
		"user_1", "", "First post! @user_2@user_3 https://www.google.com/", "post", "")
	assert.Nil(t, err)
	assert.Equal(t, "Compose successfully!", s)

	s, err = wc.ComposePost(
		"user_1", "", "Second post! https://www.bing.com/ @user_2", "repost", "")
	assert.Nil(t, err)
	assert.Equal(t, "Compose successfully!", s)

	// check timeline
	repl, err := wc.ReadTimeline("1", "2")
	assert.Nil(t, err)
	assert.Equal(t, "Timeline successfully!", repl["message"].(string))
	assert.Equal(t, "user_1; user_1; ", repl["creators"].(string))
	contents := strings.Split(repl["contents"].(string), "; ")
	assert.True(t, strings.HasPrefix(contents[0], "Second post!"))
	assert.True(t, strings.HasPrefix(contents[1], "First post! @user_2@user_3"))

	// check hometimeline
	repl, err = wc.ReadHome("15", "2")
	assert.Nil(t, err)
	assert.Equal(t, "Timeline successfully!", repl["message"].(string))
	assert.Equal(t, "user_1; user_1; ", repl["creators"].(string))
	contents = strings.Split(repl["contents"].(string), "; ")
	assert.True(t, strings.HasPrefix(contents[0], "Second post!"))
	assert.True(t, strings.HasPrefix(contents[1], "First post! @user_2@user_3"))

	repl, err = wc.ReadHome("2", "2")
	assert.Nil(t, err)
	assert.Equal(t, "Timeline successfully!", repl["message"].(string))
	assert.Equal(t, "user_1; user_1; ", repl["creators"].(string))

	repl, err = wc.ReadHome("3", "")
	assert.Nil(t, err)
	assert.Equal(t, "Timeline successfully!", repl["message"].(string))
	assert.True(t, strings.HasPrefix(repl["contents"].(string), "First post!"))
}

func testSeqMixInner(t *testing.T, wc *sn.WebClnt) {
	rmsg, err := wc.StartRecording()
	assert.Nil(t, err)
	assert.Equal(t, "Started recording!", rmsg)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	N := 1000
	t0 := time.Now()
	for i := 0; i < N; i++ {
		if i % (N/10) == 0 {
			dbg.DPrintf(dbg.TEST, "Check point at %v: %v", i, time.Since(t0).Microseconds())
		}
		randOps(t, wc, r)
	}
	dbg.DPrintf(dbg.TEST, "Final time: %v", time.Since(t0).Microseconds())
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

func randOps(t *testing.T, wc *sn.WebClnt, r *rand.Rand) {
	ratio := float64(r.Intn(10000))/10000
	if ratio < COMPOSE_RATIO {
		randCompose(t, wc, r)
	} else if ratio < COMPOSE_RATIO + HOME_RATIO {
		randReadHome(t, wc, r)
	} else {
		randReadTimeline(t, wc, r)
	}
}

func testLoadgenInner(t *testing.T, wc *sn.WebClnt) {
	lg := loadgen.MakeLoadGenerator(
		LOAD_DUR * time.Second, LOAD_MAX_RPS, func(r *rand.Rand) { randOps(t, wc, r) })
	lg.Calibrate()
	rmsg, err := wc.StartRecording()
	assert.Nil(t, err)
	assert.Equal(t, "Started recording!", rmsg)
	lg.Run()
	if lg != nil {
		lg.Stats()
	}
}

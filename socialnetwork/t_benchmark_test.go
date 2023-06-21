package socialnetwork_test

import (
	"testing"
	"sigmaos/test"
	dbg "sigmaos/debug"
	sn "sigmaos/socialnetwork"
	sp "sigmaos/sigmap"
	"github.com/stretchr/testify/assert"
	"strings"
	"strconv"
	"flag"
	"fmt"
	"time"
)

var K8S_ADDR string
var BENCH_TEST bool

func init() {
	flag.StringVar(&K8S_ADDR, "k8saddr", "", "Addr of k8s frontend.")
	flag.BoolVar(&BENCH_TEST, "benchtest", false, "Is this a benchmark test?")
}

func setupK8sState(tssn *TstateSN) {
	// Advertise server address
	p := sn.JobHTTPAddrsPath(tssn.jobname)
	mnt := sp.MkMountService(sp.MkTaddrs([]string{K8S_ADDR}))
	if err := tssn.MountService(p, mnt); err != nil {
		dbg.DFatalf("MountService %v", err)
	}
}

func makeFullTstateSN(t *testing.T) *TstateSN {
	tssn := makeTstateSN(t, []sn.Srv{
		sn.Srv{"socialnetwork-user", test.Overlays, 1},
		sn.Srv{"socialnetwork-graph", test.Overlays, 1},
		sn.Srv{"socialnetwork-post", test.Overlays, 1},
		sn.Srv{"socialnetwork-timeline", test.Overlays, 1},
		sn.Srv{"socialnetwork-home", test.Overlays, 1},
		sn.Srv{"socialnetwork-url", test.Overlays, 1},
		sn.Srv{"socialnetwork-text", test.Overlays, 1},
		sn.Srv{"socialnetwork-compose", test.Overlays, 1},
		sn.Srv{"socialnetwork-frontend", test.Overlays, 1}}, NSHARD)
	tssn.dbu.InitUser()
	tssn.dbu.InitGraph()
	return tssn
}

func TestBenchmarkSigmaOS(t *testing.T) {
	if !BENCH_TEST {
		dbg.DPrintf(dbg.ALWAYS, "Skipping benchmark test")
		return
	}
	tssn := makeFullTstateSN(t)
	wc := sn.MakeWebClnt(tssn.FsLib, tssn.jobname)

	// run tests
	testBenchmarkInner(t, wc)

	//stop server
	assert.Nil(t, tssn.Shutdown())
}

func testBenchmarkInner(t *testing.T, wc *sn.WebClnt) {
	rmsg, err := wc.StartRecording()
	assert.Nil(t, err)
	assert.Equal(t, "Started recording!", rmsg)
	dbg.DPrintf(dbg.TEST, "Start time : %v", time.Now().String())
	t0 := time.Now()
	// log in
	users := make([]string, sn.NUSER)
	for i := 0; i < sn.NUSER; i++ {
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
		meStr := users[i%sn.NUSER] 
		msg := fmt.Sprintf(
			"My post #%d! @user_%d@user_%d https://www.google.com/?search=%d", 
			i, (i+2)%sn.NUSER, (i+7)%sn.NUSER, i)
		s, err := wc.ComposePost("user_"+meStr, "", msg, "post")
		assert.Nil(t, err)
		assert.Equal(t, "Compose successfully!", s)
		// check timeline and hometimeline
		for j := 1; j <=5; j++ {
			_, err := wc.ReadHome(users[(i+j)%sn.NUSER], "1")
			assert.Nil(t, err)
			replTl, err := wc.ReadTimeline(meStr, "1")
			assert.Nil(t, err)
			assert.Equal(t, "Timeline successfully!", replTl["message"].(string))
		}
	}
	dbg.DPrintf(dbg.TEST, "Finish time : %v", time.Since(t0).Microseconds())
}

func TestFrontendK8s(t *testing.T) {
	if K8S_ADDR == "" {
		dbg.DPrintf(dbg.ALWAYS, "No k8s addr supplied")
		return
	}
	tssn := makeTstateSN(t, nil, 0)
	setupK8sState(tssn)
	wc := sn.MakeWebClnt(tssn.FsLib, tssn.jobname)

	// run tests
	testFrontEndInner(t, wc)

	//stop server
	assert.Nil(t, tssn.Shutdown())
}

func TestFrontend(t *testing.T) {
	// start server and creat web client
	tssn := makeFullTstateSN(t)
	wc := sn.MakeWebClnt(tssn.FsLib, tssn.jobname)

	// run tests
	testFrontEndInner(t, wc)

	//stop server
	assert.Nil(t, tssn.Shutdown())
}

func testFrontEndInner(t *testing.T, wc *sn.WebClnt) {
	// log in
	s, err := wc.Login("user_0", "p_user_0")
	assert.Nil(t, err)
	assert.Equal(t, "Login successfully!", s)

	s, err = wc.Login("user_1", "p_user_0")
	assert.Nil(t, err)
	assert.Equal(t, "Failed. Please check your username and password. ", s)

	// compose posts
	s, err = wc.ComposePost(
		"user_1", "", "First post! @user_2@user_3 https://www.google.com/", "post")
	assert.Nil(t, err)
	assert.Equal(t, "Compose successfully!", s)

	s, err = wc.ComposePost(
		"user_1", "", "Second post! https://www.bing.com/ @user_2", "repost")
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
	repl, err = wc.ReadHome("0", "2")
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



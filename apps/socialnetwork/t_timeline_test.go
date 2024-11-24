package socialnetwork_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	sn "sigmaos/apps/socialnetwork"
	"sigmaos/apps/socialnetwork/proto"
	"sigmaos/linuxsched"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/test"
)

func createNPosts(t *testing.T, rpcc *rpcclnt.RPCClnt, N int, userid int64) []*proto.Post {
	posts := make([]*proto.Post, N)
	for i := 0; i < N; i++ {
		posts[i] = &proto.Post{
			Postid:       int64(i),
			Posttype:     proto.POST_TYPE_POST,
			Timestamp:    int64(10000 + i),
			Creator:      userid,
			Text:         fmt.Sprintf("Post Number %v", i+1),
			Urls:         []string{"xxxxx"},
			Usermentions: []int64{userid*10 + int64(i+1)},
		}
		arg_store := proto.StorePostRequest{Post: posts[i]}
		res_store := proto.StorePostResponse{}
		assert.Nil(t, rpcc.RPC("PostSrv.StorePost", &arg_store, &res_store))
		assert.Equal(t, "OK", res_store.Ok)
	}
	return posts
}

func writeTimeline(t *testing.T, rpcc *rpcclnt.RPCClnt, post *proto.Post, userid int64) {
	arg_write := proto.WriteTimelineRequest{
		Userid: userid, Postid: post.Postid, Timestamp: post.Timestamp}
	res_write := proto.WriteTimelineResponse{}
	assert.Nil(t, rpcc.RPC("TimelineSrv.WriteTimeline", &arg_write, &res_write))
	assert.Equal(t, "OK", res_write.Ok)
}

func writeHomeTimeline(t *testing.T, rpcc *rpcclnt.RPCClnt, post *proto.Post, userid int64) {
	mentionids := make([]int64, 0)
	for _, mention := range post.Usermentions {
		mentionids = append(mentionids, mention)
	}
	arg_write := proto.WriteHomeTimelineRequest{
		Userid: userid, Postid: post.Postid,
		Timestamp: post.Timestamp, Usermentionids: mentionids}
	res_write := proto.WriteTimelineResponse{}
	assert.Nil(t, rpcc.RPC("HomeSrv.WriteHomeTimeline", &arg_write, &res_write))
	assert.Equal(t, "OK", res_write.Ok)
}

func TestTimeline(t *testing.T) {
	// Bail out early if machine has too many cores (which messes with the cgroups setting)
	if !assert.False(t, linuxsched.GetNCores() > 10, "Test will fail because machine has >10 cores, which causes cgroups settings to fail") {
		return
	}
	// start server
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	tssn, err := newTstateSN(t1, []sn.Srv{
		sn.Srv{"socialnetwork-post", nil, 1000},
		sn.Srv{"socialnetwork-timeline", nil, 1000}}, NCACHESRV)
	defer assert.Nil(t, tssn.Shutdown())
	if err != nil {
		return
	}
	snCfg := tssn.snCfg

	// create RPC clients for posts and timelines
	trpcc, err := sprpcclnt.NewRPCClnt(snCfg.FsLib, sn.SOCIAL_NETWORK_TIMELINE)
	if !assert.Nil(t, err, "Err make rpcclnt: %v", err) {
		return
	}
	prpcc, err := sprpcclnt.NewRPCClnt(snCfg.FsLib, sn.SOCIAL_NETWORK_POST)
	if !assert.Nil(t, err, "Err make rpcclnt: %v", err) {
		return
	}

	// create and store N posts
	NPOST, userid := 4, int64(200)
	posts := createNPosts(t, prpcc, NPOST, userid)

	// write posts 0 to N/2 to timeline
	for i := 0; i < NPOST/2; i++ {
		writeTimeline(t, trpcc, posts[i], userid)
	}
	arg_read := proto.ReadTimelineRequest{Userid: userid, Start: int32(0), Stop: int32(1)}
	res_read := proto.ReadTimelineResponse{}
	assert.Nil(t, trpcc.RPC("TimelineSrv.ReadTimeline", &arg_read, &res_read))
	assert.Equal(t, 1, len(res_read.Posts))
	assert.Equal(t, "OK", res_read.Ok)
	assert.True(t, IsPostEqual(posts[NPOST/2-1], res_read.Posts[0]))
	arg_read.Stop = int32(NPOST)
	assert.Nil(t, trpcc.RPC("TimelineSrv.ReadTimeline", &arg_read, &res_read))
	assert.Equal(t, "OK", res_read.Ok)

	// write post N/2 to N to timeline
	for i := NPOST / 2; i < NPOST; i++ {
		writeTimeline(t, trpcc, posts[i], userid)
	}
	arg_read.Start = int32(1)
	assert.Nil(t, trpcc.RPC("TimelineSrv.ReadTimeline", &arg_read, &res_read))
	assert.Equal(t, NPOST-1, len(res_read.Posts))
	assert.Equal(t, "OK", res_read.Ok)
	for i, tlpost := range res_read.Posts {
		// posts should be in reverse order
		assert.True(t, IsPostEqual(posts[NPOST-i-2], tlpost))
	}
}

func TestHome(t *testing.T) {
	// Bail out early if machine has too many cores (which messes with the cgroups setting)
	if !assert.False(t, linuxsched.GetNCores() > 10, "Test will fail because machine has >10 cores, which causes cgroups settings to fail") {
		return
	}
	// start server
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	tssn, err := newTstateSN(t1, []sn.Srv{
		sn.Srv{"socialnetwork-user", nil, 1000},
		sn.Srv{"socialnetwork-graph", nil, 1000},
		sn.Srv{"socialnetwork-post", nil, 1000},
		sn.Srv{"socialnetwork-home", nil, 1000}}, NCACHESRV)
	defer assert.Nil(t, tssn.Shutdown())
	if err != nil {
		return
	}
	snCfg := tssn.snCfg
	tssn.dbu.InitGraph()

	// create RPC clients for posts and timelines
	hrpcc, err := sprpcclnt.NewRPCClnt(snCfg.FsLib, sn.SOCIAL_NETWORK_HOME)
	if !assert.Nil(t, err, "Err make rpcclnt: %v", err) {
		return
	}
	prpcc, err := sprpcclnt.NewRPCClnt(snCfg.FsLib, sn.SOCIAL_NETWORK_POST)
	if !assert.Nil(t, err, "Err make rpcclnt: %v", err) {
		return
	}

	// create and store N posts
	NPOST, userid := 3, int64(1)
	posts := createNPosts(t, prpcc, NPOST, userid)

	// write to home timelines and check
	for i := 0; i < NPOST; i++ {
		writeHomeTimeline(t, hrpcc, posts[i], userid)
	}
	// first post is on user 0 and 11's home timelines
	// second post is on user 0 and 12's home timelines ......
	arg_read := proto.ReadTimelineRequest{Userid: int64(0), Start: int32(0), Stop: int32(NPOST)}
	res_read := proto.ReadTimelineResponse{}
	assert.Nil(t, hrpcc.RPC("HomeSrv.ReadHomeTimeline", &arg_read, &res_read))
	assert.Equal(t, NPOST, len(res_read.Posts))
	for i, post := range res_read.Posts {
		// posts should be in reverse order
		assert.True(t, IsPostEqual(posts[NPOST-i-1], post))
	}
	arg_read.Stop = int32(1)
	for i := 0; i < NPOST; i++ {
		arg_read.Userid = userid*10 + int64(i+1)
		assert.Nil(t, hrpcc.RPC("HomeSrv.ReadHomeTimeline", &arg_read, &res_read))
		assert.Equal(t, 1, len(res_read.Posts))
		assert.True(t, IsPostEqual(posts[i], res_read.Posts[0]))
	}
}

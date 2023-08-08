package socialnetwork_test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"sigmaos/cachesrv"
	"sigmaos/fslib"
	"sigmaos/rpcclnt"
	sp "sigmaos/sigmap"
	sn "sigmaos/socialnetwork"
	"sigmaos/socialnetwork/proto"
	"sigmaos/test"
	"testing"
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
		assert.Nil(t, rpcc.RPC("Post.StorePost", &arg_store, &res_store))
		assert.Equal(t, "OK", res_store.Ok)
	}
	return posts
}

func writeTimeline(t *testing.T, rpcc *rpcclnt.RPCClnt, post *proto.Post, userid int64) {
	arg_write := proto.WriteTimelineRequest{
		Userid: userid, Postid: post.Postid, Timestamp: post.Timestamp}
	res_write := proto.WriteTimelineResponse{}
	assert.Nil(t, rpcc.RPC("Timeline.WriteTimeline", &arg_write, &res_write))
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
	assert.Nil(t, rpcc.RPC("Home.WriteHomeTimeline", &arg_write, &res_write))
	assert.Equal(t, "OK", res_write.Ok)
}

func TestTimeline(t *testing.T) {
	// start server
	tssn := makeTstateSN(t, []sn.Srv{
		sn.Srv{"socialnetwork-post", test.Overlays, 2},
		sn.Srv{"socialnetwork-timeline", test.Overlays, 2}}, cachesrv.NSHARD)
	snCfg := tssn.snCfg

	// create RPC clients for posts and timelines
	trpcc, err := rpcclnt.MkRPCClnt([]*fslib.FsLib{snCfg.FsLib}, sp.SOCIAL_NETWORK_TIMELINE)
	assert.Nil(t, err)
	prpcc, err := rpcclnt.MkRPCClnt([]*fslib.FsLib{snCfg.FsLib}, sp.SOCIAL_NETWORK_POST)
	assert.Nil(t, err)

	// create and store N posts
	NPOST, userid := 4, int64(200)
	posts := createNPosts(t, prpcc, NPOST, userid)

	// write posts 0 to N/2 to timeline
	for i := 0; i < NPOST/2; i++ {
		writeTimeline(t, trpcc, posts[i], userid)
	}
	arg_read := proto.ReadTimelineRequest{Userid: userid, Start: int32(0), Stop: int32(1)}
	res_read := proto.ReadTimelineResponse{}
	assert.Nil(t, trpcc.RPC("Timeline.ReadTimeline", &arg_read, &res_read))
	assert.Equal(t, 1, len(res_read.Posts))
	assert.Equal(t, "OK", res_read.Ok)
	assert.True(t, IsPostEqual(posts[NPOST/2-1], res_read.Posts[0]))
	arg_read.Stop = int32(NPOST)
	assert.Nil(t, trpcc.RPC("Timeline.ReadTimeline", &arg_read, &res_read))
	assert.Equal(t, "OK", res_read.Ok)

	// write post N/2 to N to timeline
	for i := NPOST / 2; i < NPOST; i++ {
		writeTimeline(t, trpcc, posts[i], userid)
	}
	arg_read.Start = int32(1)
	assert.Nil(t, trpcc.RPC("Timeline.ReadTimeline", &arg_read, &res_read))
	assert.Equal(t, NPOST-1, len(res_read.Posts))
	assert.Equal(t, "OK", res_read.Ok)
	for i, tlpost := range res_read.Posts {
		// posts should be in reverse order
		assert.True(t, IsPostEqual(posts[NPOST-i-2], tlpost))
	}

	//stop server
	assert.Nil(t, tssn.Shutdown())
}

func TestHome(t *testing.T) {
	// start server
	tssn := makeTstateSN(t, []sn.Srv{
		sn.Srv{"socialnetwork-user", test.Overlays, 2},
		sn.Srv{"socialnetwork-graph", test.Overlays, 2},
		sn.Srv{"socialnetwork-post", test.Overlays, 2},
		sn.Srv{"socialnetwork-home", test.Overlays, 2}}, cachesrv.NSHARD)
	snCfg := tssn.snCfg
	tssn.dbu.InitGraph()

	// create RPC clients for posts and timelines
	hrpcc, err := rpcclnt.MkRPCClnt([]*fslib.FsLib{snCfg.FsLib}, sp.SOCIAL_NETWORK_HOME)
	assert.Nil(t, err)
	prpcc, err := rpcclnt.MkRPCClnt([]*fslib.FsLib{snCfg.FsLib}, sp.SOCIAL_NETWORK_POST)
	assert.Nil(t, err)

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
	assert.Nil(t, hrpcc.RPC("Home.ReadHomeTimeline", &arg_read, &res_read))
	assert.Equal(t, NPOST, len(res_read.Posts))
	for i, post := range res_read.Posts {
		// posts should be in reverse order
		assert.True(t, IsPostEqual(posts[NPOST-i-1], post))
	}
	arg_read.Stop = int32(1)
	for i := 0; i < NPOST; i++ {
		arg_read.Userid = userid*10 + int64(i+1)
		assert.Nil(t, hrpcc.RPC("Home.ReadHomeTimeline", &arg_read, &res_read))
		assert.Equal(t, 1, len(res_read.Posts))
		assert.True(t, IsPostEqual(posts[i], res_read.Posts[0]))
	}

	//stop server
	assert.Nil(t, tssn.Shutdown())
}

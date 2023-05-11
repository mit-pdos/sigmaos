package socialnetwork_test

import (
	"testing"
	"sigmaos/test"
	"sigmaos/fslib"
	sn "sigmaos/socialnetwork"
	sp "sigmaos/sigmap"
	"sigmaos/socialnetwork/proto"
	"sigmaos/protdevclnt"
	"github.com/stretchr/testify/assert"
	"fmt"
)

func createNPosts(t *testing.T, pdc *protdevclnt.ProtDevClnt, N int, userid int64) []*proto.Post {
	user := proto.UserRef{Userid: userid, Username: "usera"} 
	url := proto.Url{Shorturl: "xxxxx", Extendedurl: "yyyyy"}
	posts := make([]*proto.Post, N)
	for i := 0; i < N; i++ {
		mention := proto.UserRef{Userid: userid*10+int64(i+1), Username: "userb"}
		posts[i] = &proto.Post{
			Postid: int64(i),
			Posttype: proto.POST_TYPE_POST,
			Timestamp: int64(10000+i),
			Creator: &user,
			Text: fmt.Sprintf("Post Number %v", i+1),
			Urls: []*proto.Url{&url},
			Usermentions: []*proto.UserRef{&mention},
		}
		arg_store := proto.StorePostRequest{Post: posts[i]}
		res_store := proto.StorePostResponse{}
		assert.Nil(t, pdc.RPC("Post.StorePost", &arg_store, &res_store))
		assert.Equal(t, "OK", res_store.Ok)
	}
	return posts
} 

func writeTimeline(t *testing.T, pdc *protdevclnt.ProtDevClnt, post *proto.Post, userid int64) {
	arg_write := proto.WriteTimelineRequest{
		Timelineitem: &proto.TimelineItem{
			Userid: userid, 
			Postid: post.Postid, 
			Timestamp: post.Timestamp}}
	res_write := proto.WriteTimelineResponse{}
	assert.Nil(t, pdc.RPC("Timeline.WriteTimeline", &arg_write, &res_write))
	assert.Equal(t, "OK", res_write.Ok)
}

func writeHomeTimeline(t *testing.T, pdc *protdevclnt.ProtDevClnt, post *proto.Post, userid int64) {
	mentionids := make([]int64, 0)
	for _, mention := range post.Usermentions {
		mentionids = append(mentionids, mention.Userid)
	}
	arg_write := proto.WriteHomeTimelineRequest{
		Timelineitem: &proto.TimelineItem{
			Userid: userid, 
			Postid: post.Postid, 
			Timestamp: post.Timestamp},
		Usermentionids: mentionids,
	}
	res_write := proto.WriteTimelineResponse{}
	assert.Nil(t, pdc.RPC("Home.WriteHomeTimeline", &arg_write, &res_write))
	assert.Equal(t, "OK", res_write.Ok)
}

func TestTimeline(t *testing.T) {
	// start server
	tssn := makeTstateSN(t, []sn.Srv{
		sn.Srv{"socialnetwork-post", test.Overlays, 2},
		sn.Srv{"socialnetwork-timeline", test.Overlays, 2}}, NSHARD)
	snCfg := tssn.snCfg

	// create RPC clients for posts and timelines
	tpdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{snCfg.FsLib}, sp.SOCIAL_NETWORK_TIMELINE)
	assert.Nil(t, err)
	ppdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{snCfg.FsLib}, sp.SOCIAL_NETWORK_POST)
	assert.Nil(t, err)

	// create and store N posts
	NPOST, userid := 4, int64(200)
	posts := createNPosts(t, ppdc, NPOST, userid)

	// write posts 0 to N/2 to timeline
	for i := 0; i < NPOST/2; i++ {
		writeTimeline(t, tpdc, posts[i], userid) 
	}
	arg_read := proto.ReadTimelineRequest{Userid: userid, Start: int32(0), Stop: int32(1)}
	res_read := proto.ReadTimelineResponse{}
	assert.Nil(t, tpdc.RPC("Timeline.ReadTimeline", &arg_read, &res_read))
	assert.Equal(t, 1, len(res_read.Posts))
	assert.Equal(t, "OK", res_read.Ok)
	assert.True(t, IsPostEqual(posts[NPOST/2-1], res_read.Posts[0]))
	arg_read.Stop = int32(NPOST)
	assert.Nil(t, tpdc.RPC("Timeline.ReadTimeline", &arg_read, &res_read))
	assert.Equal(t, 
		fmt.Sprintf("Cannot process start=0 end=%v for %v items", NPOST, NPOST/2), res_read.Ok) 

	// write post N/2 to N to timeline 	
	for i := NPOST/2; i < NPOST; i++ {
		writeTimeline(t, tpdc, posts[i], userid) 
	}
	arg_read.Start = int32(1)
	assert.Nil(t, tpdc.RPC("Timeline.ReadTimeline", &arg_read, &res_read))
	assert.Equal(t, NPOST-1, len(res_read.Posts))
	assert.Equal(t, "OK", res_read.Ok)
	for i, tlpost := range(res_read.Posts) {
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
		sn.Srv{"socialnetwork-home", test.Overlays, 2}}, NSHARD)
	snCfg := tssn.snCfg
	tssn.dbu.InitGraph()

	// create RPC clients for posts and timelines
	hpdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{snCfg.FsLib}, sp.SOCIAL_NETWORK_HOME)
	assert.Nil(t, err)
	ppdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{snCfg.FsLib}, sp.SOCIAL_NETWORK_POST)
	assert.Nil(t, err)

	// create and store N posts
	NPOST, userid := 3, int64(1)
	posts := createNPosts(t, ppdc, NPOST, userid)
	
	// write to home timelines and check
	for i := 0; i < NPOST; i++ {
		writeHomeTimeline(t, hpdc, posts[i], userid) 
	}
	// first post is on user 0 and 11's home timelines
	// second post is on user 0 and 12's home timelines ......
	arg_read := proto.ReadTimelineRequest{Userid: int64(0), Start: int32(0), Stop: int32(NPOST)}
	res_read := proto.ReadTimelineResponse{}
	assert.Nil(t, hpdc.RPC("Home.ReadHomeTimeline", &arg_read, &res_read))
	assert.Equal(t, NPOST, len(res_read.Posts))
	for i, post := range(res_read.Posts) {
		// posts should be in reverse order
		assert.True(t, IsPostEqual(posts[NPOST-i-1], post))
	}
	arg_read.Stop = int32(1)
	for i := 0; i < NPOST; i++ {
		arg_read.Userid = userid*10+int64(i+1)
		assert.Nil(t, hpdc.RPC("Home.ReadHomeTimeline", &arg_read, &res_read))
		assert.Equal(t, 1, len(res_read.Posts))
		assert.True(t, IsPostEqual(posts[i], res_read.Posts[0]))
	}

	//stop server
	assert.Nil(t, tssn.Shutdown())
}

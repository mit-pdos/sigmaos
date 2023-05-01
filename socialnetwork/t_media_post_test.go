package socialnetwork_test

import (
	"testing"
	"sigmaos/test"
	sn "sigmaos/socialnetwork"
	sp "sigmaos/sigmap"
	"sigmaos/socialnetwork/proto"
	"sigmaos/protdevclnt"
	"github.com/stretchr/testify/assert"
)

func TestPostEncode(t *testing.T) {
	// encode 
	postid := int64(377)
	creator := proto.UserRef{Userid: int64(200), Username: "usera"} 
	mention1 := proto.UserRef{Userid: int64(201), Username: "userb"} 
	mention2 := proto.UserRef{Userid: int64(202), Username: "userc"} 
	mentions := []*proto.UserRef{&mention1, &mention2}
	url := proto.Url{Shorturl: "XXXXX", Extendedurl: "YYYYY"}
	post := proto.Post{
		Postid: postid,
		Posttype: proto.POST_TYPE_POST,
		Timestamp: int64(543210),
		Creator: &creator,
		Text: "My First Post",
		Usermentions: mentions,
		Urls: []*proto.Url{&url},
	}
	encode, err := sn.EncodePost(post) 
	assert.Nil(t, err)

	// decode
	var postDecoded proto.Post
	assert.Nil(t, sn.DecodePost(encode, &postDecoded))
	assert.Equal(t, post.Text, postDecoded.Text)
	assert.Equal(t, postid, postDecoded.Postid)
	assert.Equal(t, post.Timestamp, postDecoded.Timestamp)
	assert.Equal(t, post.Creator, postDecoded.Creator)
	assert.Equal(t, post.Usermentions, postDecoded.Usermentions)
	assert.Equal(t, post.Urls, postDecoded.Urls)
	assert.Equal(t, 0, len(postDecoded.Medias))
}

func TestPost(t *testing.T) {
	// start server
	tssn := makeTstateSN(t, []sn.Srv{sn.Srv{"socialnetwork-post", test.Overlays, 2}}, NSHARD)
	snCfg := tssn.snCfg

	// create a RPC client and query
	pdc, err := protdevclnt.MkProtDevClnt(snCfg.FsLib, sp.SOCIAL_NETWORK_POST)
	assert.Nil(t, err, "RPC client should be created properly")

	// create two posts
	usera := proto.UserRef{Userid: int64(200), Username: "usera"} 
	userb := proto.UserRef{Userid: int64(201), Username: "userb"} 
	userc := proto.UserRef{Userid: int64(202), Username: "userc"} 
	url1 := proto.Url{Shorturl: "xxxxx", Extendedurl: "yyyyy"}
	url2 := proto.Url{Shorturl: "XXXXX", Extendedurl: "YYYYY"}
	media := proto.MediaRef{Mediaid: int64(777), Mediatype: "video"}
	post1 := proto.Post{
		Postid: int64(1),
		Posttype: proto.POST_TYPE_POST,
		Timestamp: int64(12345),
		Creator: &usera,
		Text: "First Post",
		Usermentions: []*proto.UserRef{&userb},
		Medias: []*proto.MediaRef{&media},
		Urls: []*proto.Url{&url1},
	}
	post2 := proto.Post{
		Postid: int64(2),
		Posttype: proto.POST_TYPE_REPOST,
		Timestamp: int64(67890),
		Creator: &usera,
		Text: "Second Post",
		Usermentions: []*proto.UserRef{&userc},
		Urls: []*proto.Url{&url2},
	}

	// store first post
	arg_store := proto.StorePostRequest{Post: &post1}
	res_store := proto.StorePostResponse{}
	assert.Nil(t, pdc.RPC("Post.StorePost", &arg_store, &res_store))
	assert.Equal(t, "OK", res_store.Ok)
	
	// check for two posts. one missing
	arg_read := proto.ReadPostsRequest{Postids: []int64{int64(1), int64(2)}}
	res_read := proto.ReadPostsResponse{}
	assert.Nil(t, pdc.RPC("Post.ReadPosts", &arg_read, &res_read))
	assert.Equal(t, "No. Missing 2.", res_read.Ok)
	
	// store second post and check for both.
	arg_store.Post = &post2
	assert.Nil(t, pdc.RPC("Post.StorePost", &arg_store, &res_store))
	assert.Equal(t, "OK", res_store.Ok)
	assert.Nil(t, pdc.RPC("Post.ReadPosts", &arg_read, &res_read))
	assert.Equal(t, "OK", res_read.Ok)
	assert.Equal(t, post1.Text, res_read.Posts[0].Text)
	assert.Equal(t, post1.Postid, res_read.Posts[0].Postid)
	assert.Equal(t, post1.Timestamp, res_read.Posts[0].Timestamp)
	assert.Equal(t, post1.Medias[0].Mediaid, res_read.Posts[0].Medias[0].Mediaid)
	assert.Equal(t, post2.Creator.Userid, res_read.Posts[1].Creator.Userid)
	assert.Equal(t, post2.Creator.Username, res_read.Posts[1].Creator.Username)
	assert.Equal(t, post2.Usermentions[0].Username, res_read.Posts[1].Usermentions[0].Username)
	assert.Equal(t, post2.Urls[0].Extendedurl, res_read.Posts[1].Urls[0].Extendedurl)

	//stop server
	assert.Nil(t, tssn.Shutdown())
}

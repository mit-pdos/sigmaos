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
	"strings"
)

func TestUrl(t *testing.T) {
	// start server
	tssn := makeTstateSN(t, []sn.Srv{sn.Srv{"socialnetwork-url", test.Overlays, 2}}, NSHARD)
	snCfg := tssn.snCfg

	// create RPC clients text
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{snCfg.FsLib}, sp.SOCIAL_NETWORK_URL)
	assert.Nil(t, err)

	// compose urls
	url1 := "http://www.google.com/q=apple"
	url2 := "https://www.bing.com"
	arg_url := proto.ComposeUrlsRequest{Extendedurls: []string{url1, url2}}
	res_url := proto.ComposeUrlsResponse{}
	assert.Nil(t, pdc.RPC("Url.ComposeUrls", &arg_url, &res_url))	
	assert.Equal(t, "OK", res_url.Ok)
	assert.Equal(t, 2, len(res_url.Urls))
	assert.Equal(t, url1, res_url.Urls[0].Extendedurl)
	assert.Equal(t, url2, res_url.Urls[1].Extendedurl)
	
	// get urls
	shortUrl1 := res_url.Urls[0].Shorturl
	shortUrl2 := res_url.Urls[1].Shorturl
	arg_get := proto.GetUrlsRequest{Shorturls: []string{shortUrl1, shortUrl2}}
	res_get := proto.GetUrlsResponse{}
	assert.Nil(t, pdc.RPC("Url.GetUrls", &arg_get, &res_get))	
	assert.Equal(t, "OK", res_get.Ok)
	assert.Equal(t, 2, len(res_get.Extendedurls))
	assert.Equal(t, url1, res_get.Extendedurls[0])
	assert.Equal(t, url2, res_get.Extendedurls[1])

	//stop server
	assert.Nil(t, tssn.Shutdown())
}

func TestText(t *testing.T) {
	// start server
	tssn := makeTstateSN(t, []sn.Srv{
		sn.Srv{"socialnetwork-user", test.Overlays, 2}, 
		sn.Srv{"socialnetwork-url", test.Overlays, 2}, 
		sn.Srv{"socialnetwork-text", test.Overlays, 2}}, NSHARD)
	snCfg := tssn.snCfg

	// create RPC clients text
	tssn.dbu.InitUser()
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{snCfg.FsLib}, sp.SOCIAL_NETWORK_TEXT)
	assert.Nil(t, err)

	// process text
	arg_text := proto.ProcessTextRequest{}
	res_text := proto.ProcessTextResponse{}
	assert.Nil(t, pdc.RPC("Text.ProcessText", &arg_text, &res_text))	
	assert.Equal(t, "Cannot process empty text.", res_text.Ok)
	
	arg_text.Text = "Hello World!"
	assert.Nil(t, pdc.RPC("Text.ProcessText", &arg_text, &res_text))	
	assert.Equal(t, "OK", res_text.Ok)
	assert.Equal(t, 0, len(res_text.Usermentions))
	assert.Equal(t, 0, len(res_text.Urls))
	assert.Equal(t, "Hello World!", res_text.Text)

	arg_text.Text = 
		"First post! @user_1@user_2 http://www.google.com/q=apple @user_4 https://www.bing.com Over!"
	assert.Nil(t, pdc.RPC("Text.ProcessText", &arg_text, &res_text))	
	assert.Equal(t, "OK", res_text.Ok)
	assert.Equal(t, 3, len(res_text.Usermentions))
	assert.Equal(t, int64(1), res_text.Usermentions[0].Userid)
	assert.Equal(t, "user_1", res_text.Usermentions[0].Username)
	assert.Equal(t, int64(2), res_text.Usermentions[1].Userid)
	assert.Equal(t, "user_2", res_text.Usermentions[1].Username)
	assert.Equal(t, int64(4), res_text.Usermentions[2].Userid)
	assert.Equal(t, "user_4", res_text.Usermentions[2].Username)
	assert.Equal(t, 2, len(res_text.Urls))
	assert.Equal(t, "user_4", res_text.Usermentions[2].Username)
	assert.Equal(t, "http://www.google.com/q=apple", res_text.Urls[0].Extendedurl)
	assert.Equal(t, "https://www.bing.com", res_text.Urls[1].Extendedurl)
	sUrl1 := res_text.Urls[0].Shorturl
	sUrl2 := res_text.Urls[1].Shorturl
	expectedText := fmt.Sprintf("First post! @user_1@user_2 %v @user_4 %v Over!", sUrl1, sUrl2)
	assert.Equal(t, expectedText, res_text.Text)

	//stop server
	assert.Nil(t, tssn.Shutdown())
}

func TestCompose(t *testing.T) {
	// start server
	tssn := makeTstateSN(t, []sn.Srv{
		sn.Srv{"socialnetwork-user", test.Overlays, 2}, 
		sn.Srv{"socialnetwork-graph", test.Overlays, 2}, 
		sn.Srv{"socialnetwork-post", test.Overlays, 2}, 
		sn.Srv{"socialnetwork-timeline", test.Overlays, 2}, 
		sn.Srv{"socialnetwork-home", test.Overlays, 2}, 
		sn.Srv{"socialnetwork-url", test.Overlays, 2}, 
		sn.Srv{"socialnetwork-text", test.Overlays, 2}, 
		sn.Srv{"socialnetwork-compose", test.Overlays, 2}}, NSHARD)
	snCfg := tssn.snCfg

	// create RPC clients text
	tssn.dbu.InitUser()
	tssn.dbu.InitGraph()
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{snCfg.FsLib}, sp.SOCIAL_NETWORK_COMPOSE)
	assert.Nil(t, err)
	tpdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{snCfg.FsLib}, sp.SOCIAL_NETWORK_TIMELINE)
	assert.Nil(t, err)
	hpdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{snCfg.FsLib}, sp.SOCIAL_NETWORK_HOME)
	assert.Nil(t, err)

	// compose empty post not allowed
	arg_compose := proto.ComposePostRequest{}
	res_compose := proto.ComposePostResponse{}
	assert.Nil(t, pdc.RPC("Compose.ComposePost", &arg_compose, &res_compose))	
	assert.Equal(t, "Cannot compose empty post!", res_compose.Ok)
	
	// compose 2 posts
	arg_compose.Posttype = proto.POST_TYPE_REPOST
	arg_compose.Username = "user_1"
	arg_compose.Userid = int64(1)
	arg_compose.Text = "First post! @user_3 http://www.google.com/q=apple"
	arg_compose.Mediaids = []int64{int64(77), int64(78)}
	arg_compose.Mediatypes = []string{"video", "picture"}
	assert.Nil(t, pdc.RPC("Compose.ComposePost", &arg_compose, &res_compose))	
	assert.Equal(t, "OK", res_compose.Ok)

	arg_compose.Posttype = proto.POST_TYPE_REPOST
	arg_compose.Username = "user_1"
	arg_compose.Userid = int64(1)
	arg_compose.Text = "Second post! https://www.bing.com/ @user_2"
	assert.Nil(t, pdc.RPC("Compose.ComposePost", &arg_compose, &res_compose))	
	assert.Equal(t, "OK", res_compose.Ok)

	// check timelines: user_1 has two items
	arg_tl := proto.ReadTimelineRequest{Userid: int64(1), Start: int32(0), Stop: int32(2)}
	res_tl := proto.ReadTimelineResponse{}
	assert.Nil(t, tpdc.RPC("Timeline.ReadTimeline", &arg_tl, &res_tl))
	assert.Equal(t, 2, len(res_tl.Posts))
	assert.Equal(t, "OK", res_tl.Ok)
	post1 := res_tl.Posts[1]
	post2 := res_tl.Posts[0]
	assert.True(t, strings.HasPrefix(post1.Text, "First post! @user_3 "))
	assert.True(t, strings.HasPrefix(post2.Text, "Second post! "))

	// check hometimelines: 
	// user_0 has two items (follower), user_0 and user_3 have one item (mentioned)
	arg_home := proto.ReadTimelineRequest{Userid: int64(0), Start: int32(0), Stop: int32(2)}
	res_home := proto.ReadTimelineResponse{}
	assert.Nil(t, hpdc.RPC("Home.ReadHomeTimeline", &arg_home, &res_home))
	assert.Equal(t, 2, len(res_home.Posts))
	assert.Equal(t, "OK", res_home.Ok)
	assert.True(t, IsPostEqual(post2, res_home.Posts[0])) 
	assert.True(t, IsPostEqual(post1, res_home.Posts[1])) 

	arg_home = proto.ReadTimelineRequest{Userid: int64(2), Start: int32(0), Stop: int32(1)}
	assert.Nil(t, hpdc.RPC("Home.ReadHomeTimeline", &arg_home, &res_home))
	assert.Equal(t, "OK", res_home.Ok)
	assert.True(t, IsPostEqual(post2, res_home.Posts[0])) 

	arg_home = proto.ReadTimelineRequest{Userid: int64(3), Start: int32(0), Stop: int32(1)}
	assert.Nil(t, hpdc.RPC("Home.ReadHomeTimeline", &arg_home, &res_home))
	assert.Equal(t, "OK", res_home.Ok)
	assert.True(t, IsPostEqual(post1, res_home.Posts[0])) 

	//stop server
	assert.Nil(t, tssn.Shutdown())
}


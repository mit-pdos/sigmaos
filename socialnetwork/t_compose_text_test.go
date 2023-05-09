package socialnetwork_test

import (
	"testing"
	"sigmaos/test"
	sn "sigmaos/socialnetwork"
	sp "sigmaos/sigmap"
	"sigmaos/socialnetwork/proto"
	"sigmaos/protdevclnt"
	"github.com/stretchr/testify/assert"
	"fmt"
)

func TestUrl(t *testing.T) {
	// start server
	tssn := makeTstateSN(t, []sn.Srv{sn.Srv{"socialnetwork-url", test.Overlays, 2}}, NSHARD)
	snCfg := tssn.snCfg

	// create RPC clients text
	pdc, err := protdevclnt.MkProtDevClnt(snCfg.FsLib, sp.SOCIAL_NETWORK_URL)
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
	pdc, err := protdevclnt.MkProtDevClnt(snCfg.FsLib, sp.SOCIAL_NETWORK_TEXT)
	assert.Nil(t, err)

	// process text
	arg_text := proto.ProcessTextRequest{}
	res_text := proto.ProcessTextResponse{}
	assert.Nil(t, pdc.RPC("Text.ProcessText", &arg_text, &res_text))	
	assert.Equal(t, "Cannot process empty text.", res_text.Ok)
	
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


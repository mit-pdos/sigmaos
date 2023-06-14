package socialnetwork_test

import (
	"testing"
	"sigmaos/test"
	sn "sigmaos/socialnetwork"
	"github.com/stretchr/testify/assert"
	"strings"
)

func TestFrontend(t *testing.T) {
	// start server
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

	// create web clients and log in
	wc := sn.MakeWebClnt(tssn.FsLib, tssn.jobname)
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

	//stop server
	assert.Nil(t, tssn.Shutdown())
}


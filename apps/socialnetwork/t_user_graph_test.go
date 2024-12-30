package socialnetwork_test

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
	"sigmaos/proc"
	"testing"

	sn "sigmaos/apps/socialnetwork"
	"sigmaos/apps/socialnetwork/proto"
	linuxsched "sigmaos/util/linux/sched"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/test"
)

func TestUser(t *testing.T) {
	// Bail out early if machine has too many cores (which messes with the cgroups setting)
	if !assert.False(t, linuxsched.GetNCores() > 10, "Test will fail because machine has >10 cores, which causes cgroups settings to fail") {
		return
	}
	// start server
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	tssn, err := newTstateSN(t1, []sn.Srv{sn.Srv{"socialnetwork-user", nil, 1000}}, NCACHESRV)
	defer assert.Nil(t, tssn.Shutdown())
	if err != nil {
		return
	}
	snCfg := tssn.snCfg

	// create a RPC client and query
	tssn.dbu.InitUser()
	rpcc, err := sprpcclnt.NewRPCClnt(snCfg.FsLib, sn.SOCIAL_NETWORK_USER)
	if !assert.Nil(t, err, "Err make rpcclnt: %v", err) {
		return
	}

	// check user
	arg_check := proto.CheckUserRequest{Usernames: []string{"test_user"}}
	res_check := proto.CheckUserResponse{}
	err = rpcc.RPC("UserSrv.CheckUser", &arg_check, &res_check)
	assert.Nil(t, err)
	assert.Equal(t, "No", res_check.Ok)
	assert.Equal(t, int64(-1), res_check.Userids[0])

	// register user
	arg_reg := proto.RegisterUserRequest{
		Firstname: "Alice", Lastname: "Test", Username: "user_0", Password: "xxyyzz"}
	res_reg := proto.UserResponse{}
	err = rpcc.RPC("UserSrv.RegisterUser", &arg_reg, &res_reg)
	assert.Nil(t, err)
	assert.Equal(t, "Username user_0 already exist", res_reg.Ok)

	arg_reg.Username = "test_user"
	err = rpcc.RPC("UserSrv.RegisterUser", &arg_reg, &res_reg)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_reg.Ok)
	created_userid := res_reg.Userid

	// check user
	arg_check.Usernames = []string{"test_user", "user_1", "user_2"}
	err = rpcc.RPC("UserSrv.CheckUser", &arg_check, &res_check)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_check.Ok)
	assert.Equal(t, created_userid, res_check.Userids[0])
	assert.Equal(t, int64(1), res_check.Userids[1])
	assert.Equal(t, int64(2), res_check.Userids[2])

	// new user login
	arg_login := proto.LoginRequest{Username: "test_user", Password: "xxyy"}
	res_login := proto.UserResponse{}
	err = rpcc.RPC("UserSrv.Login", &arg_login, &res_login)
	assert.Nil(t, err)
	assert.Equal(t, "Login Failure.", res_login.Ok)

	arg_login.Password = "xxyyzz"
	err = rpcc.RPC("UserSrv.Login", &arg_login, &res_login)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_login.Ok)

	// verify cache contents
	user := &sn.User{}
	cacheItem := &proto.CacheItem{}
	err = snCfg.CacheClnt.Get(sn.USER_CACHE_PREFIX+"test_user", cacheItem)
	assert.Nil(t, err)
	bson.Unmarshal(cacheItem.Val, user)
	assert.Equal(t, "Alice", user.Firstname)
	assert.Equal(t, "Test", user.Lastname)
	assert.Equal(t, created_userid, user.Userid)
}

func TestGraph(t *testing.T) {
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
		sn.Srv{"socialnetwork-graph", nil, 1000}}, NCACHESRV)
	defer assert.Nil(t, tssn.Shutdown())
	if err != nil {
		return
	}
	snCfg := tssn.snCfg

	// create a RPC client and query
	tssn.dbu.InitGraph()
	rpcc, err := sprpcclnt.NewRPCClnt(snCfg.FsLib, sn.SOCIAL_NETWORK_GRAPH)
	if !assert.Nil(t, err, "Err make rpcclnt: %v", err) {
		return
	}

	// get follower and followee list
	arg_get_fler := proto.GetFollowersRequest{}
	arg_get_fler.Followeeid = 0
	res_get := proto.GraphGetResponse{}
	err = rpcc.RPC("GraphSrv.GetFollowers", &arg_get_fler, &res_get)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_get.Ok)
	assert.Equal(t, 0, len(res_get.Userids)) // user 0 has no follower

	arg_get_flee := proto.GetFolloweesRequest{}
	arg_get_flee.Followerid = 1
	err = rpcc.RPC("GraphSrv.GetFollowees", &arg_get_flee, &res_get)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_get.Ok)
	assert.Equal(t, 1, len(res_get.Userids))
	assert.Equal(t, int64(2), res_get.Userids[0]) // user 1 has one followee user 2

	// Follow
	arg_follow := proto.FollowRequest{}
	arg_follow.Followerid = 1
	arg_follow.Followeeid = 0
	res_update := proto.GraphUpdateResponse{}
	err = rpcc.RPC("GraphSrv.Follow", &arg_follow, &res_update) // user 1 is now following user 0
	assert.Nil(t, err, "Follow error :%v", err)
	assert.Equal(t, "OK", res_update.Ok)

	err = rpcc.RPC("GraphSrv.GetFollowers", &arg_get_fler, &res_get)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_get.Ok)
	assert.Equal(t, 1, len(res_get.Userids))
	assert.Equal(t, int64(1), res_get.Userids[0]) // user 0 has one follower user 1

	err = rpcc.RPC("GraphSrv.GetFollowees", &arg_get_flee, &res_get)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_get.Ok)
	assert.Equal(t, 2, len(res_get.Userids))
	assert.Equal(t, int64(2), res_get.Userids[0]) // user 1 has two followees user 0 & 2
	assert.Equal(t, int64(0), res_get.Userids[1]) // user 1 has two followees user 0 & 2

	// Unfollow
	arg_unfollow := proto.UnfollowRequest{}
	arg_unfollow.Followerid = 1
	arg_unfollow.Followeeid = 0
	err = rpcc.RPC("GraphSrv.Unfollow", &arg_unfollow, &res_update) // user 1 is now unfollowing user 0
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_update.Ok)

	err = rpcc.RPC("GraphSrv.GetFollowers", &arg_get_fler, &res_get)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_get.Ok)
	assert.Equal(t, 0, len(res_get.Userids)) // user 0 now again has no follower

	err = rpcc.RPC("GraphSrv.GetFollowees", &arg_get_flee, &res_get)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_get.Ok)
	assert.Equal(t, 1, len(res_get.Userids))
	assert.Equal(t, int64(2), res_get.Userids[0]) // user 1 now again has one followee user 2
}

func TestUserAndGraph(t *testing.T) {
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
		sn.Srv{"socialnetwork-graph", nil, 1000}}, NCACHESRV)
	defer assert.Nil(t, tssn.Shutdown())
	if err != nil {
		return
	}
	tssn.dbu.InitGraph()
	tssn.dbu.InitUser()
	snCfg := tssn.snCfg
	rpcc, err := sprpcclnt.NewRPCClnt(snCfg.FsLib, sn.SOCIAL_NETWORK_USER)
	if !assert.Nil(t, err, "Err make rpcclnt: %v", err) {
		return
	}
	grpcc, err := sprpcclnt.NewRPCClnt(snCfg.FsLib, sn.SOCIAL_NETWORK_GRAPH)
	if !assert.Nil(t, err, "Err make rpcclnt: %v", err) {
		return
	}

	// Create two users Alice and Bob
	arg_reg1 := proto.RegisterUserRequest{
		Firstname: "Alice", Lastname: "Test", Username: "atest", Password: "xyz"}
	arg_reg2 := proto.RegisterUserRequest{
		Firstname: "Bob", Lastname: "Test", Username: "btest", Password: "zyx"}
	res_reg := proto.UserResponse{}
	err = rpcc.RPC("UserSrv.RegisterUser", &arg_reg1, &res_reg)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_reg.Ok)
	auserid := res_reg.Userid
	err = rpcc.RPC("UserSrv.RegisterUser", &arg_reg2, &res_reg)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_reg.Ok)
	buserid := res_reg.Userid

	// Alice follows Bob
	arg_follow := proto.FollowWithUnameRequest{}
	arg_follow.Followeruname = "atest"
	arg_follow.Followeeuname = "btest"
	res_update := proto.GraphUpdateResponse{}
	err = grpcc.RPC("GraphSrv.FollowWithUname", &arg_follow, &res_update)
	assert.Nil(t, err, "Error is: %v", err)
	assert.Equal(t, "OK", res_update.Ok)

	arg_get_fler := proto.GetFollowersRequest{}
	arg_get_fler.Followeeid = buserid
	res_get := proto.GraphGetResponse{}
	err = grpcc.RPC("GraphSrv.GetFollowers", &arg_get_fler, &res_get)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_get.Ok)
	assert.Equal(t, 1, len(res_get.Userids))
	assert.Equal(t, auserid, res_get.Userids[0])

	arg_get_flee := proto.GetFolloweesRequest{}
	arg_get_flee.Followerid = auserid
	err = grpcc.RPC("GraphSrv.GetFollowees", &arg_get_flee, &res_get)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_get.Ok)
	assert.Equal(t, 1, len(res_get.Userids))
	assert.Equal(t, buserid, res_get.Userids[0])

	// Alice unfollows Bob
	arg_unfollow := proto.UnfollowWithUnameRequest{}
	arg_unfollow.Followeruname = "atest"
	arg_unfollow.Followeeuname = "btest"
	err = grpcc.RPC("GraphSrv.UnfollowWithUname", &arg_unfollow, &res_update)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_update.Ok)

	err = grpcc.RPC("GraphSrv.GetFollowers", &arg_get_fler, &res_get)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_get.Ok)
	assert.Equal(t, 0, len(res_get.Userids))

	err = grpcc.RPC("GraphSrv.GetFollowees", &arg_get_flee, &res_get)
	assert.Nil(t, err)
	assert.Equal(t, "OK", res_get.Ok)
	assert.Equal(t, 0, len(res_get.Userids))
}

func testRPCTime(t *testing.T, mcpu proc.Tmcpu) {
	// start server
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	tssn, err := newTstateSN(t1, []sn.Srv{sn.Srv{"socialnetwork-user", nil, mcpu}}, 1)
	defer assert.Nil(t, tssn.Shutdown())
	if err != nil {
		return
	}
	snCfg := tssn.snCfg

	// create a RPC client and query
	tssn.dbu.InitUser()
	rpcc, err := sprpcclnt.NewRPCClnt(snCfg.FsLib, sn.SOCIAL_NETWORK_USER)
	if !assert.Nil(t, err, "Err make rpcclnt: %v", err) {
		return
	}

	// check user
	arg_check := proto.CheckUserRequest{Usernames: []string{"user_1"}}
	res_check := proto.CheckUserResponse{}
	for i := 1; i < 5001; i++ {
		assert.Nil(t, rpcc.RPC("UserSrv.CheckUser", &arg_check, &res_check))
		assert.Equal(t, "OK", res_check.Ok)
		assert.Equal(t, int64(1), res_check.Userids[0])
	}
}

func TestRPCTimeOneMachine(t *testing.T) {
	// Bail out early if machine has too many cores (which messes with the cgroups setting)
	if !assert.False(t, linuxsched.GetNCores() > 10, "Test will fail because machine has >10 cores, which causes cgroups settings to fail") {
		return
	}
	testRPCTime(t, 1000)
}

func TestRPCTimeTwoMachines(t *testing.T) {
	// Bail out early if machine has too many cores (which messes with the cgroups setting)
	if !assert.False(t, linuxsched.GetNCores() > 10, "Test will fail because machine has >10 cores, which causes cgroups settings to fail") {
		return
	}
	testRPCTime(t, 2500)
}

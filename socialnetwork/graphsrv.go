package socialnetwork

import (
	"fmt"

	"gopkg.in/mgo.v2/bson"
	"sigmaos/cache"
	"sigmaos/cachedsvcclnt"
	dbg "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/mongoclnt"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/rpcclnt"
	"sigmaos/sigmarpcchan"
	"sigmaos/sigmasrv"
	"sigmaos/socialnetwork/proto"
	"strconv"
)

// YH:
// Social Graph service for social network
// for now we use sql instead of MongoDB

const (
	GRAPH_QUERY_OK        = "OK"
	FOLLOWER_CACHE_PREFIX = "followers_"
	FOLLOWEE_CACHE_PREFIX = "followees_"
)

type GraphSrv struct {
	mongoc *mongoclnt.MongoClnt
	cachec *cachedsvcclnt.CachedSvcClnt
	userc  *rpcclnt.RPCClnt
}

func RunGraphSrv(public bool, jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_GRAPH, "Creating graph service\n")
	gsrv := &GraphSrv{}
	ssrv, err := sigmasrv.NewSigmaSrvPublic(SOCIAL_NETWORK_GRAPH, gsrv, proc.GetProcEnv(), public)
	if err != nil {
		return err
	}
	mongoc, err := mongoclnt.NewMongoClnt(ssrv.MemFs.SigmaClnt().FsLib)
	if err != nil {
		return err
	}
	mongoc.EnsureIndex(SN_DB, GRAPH_FLWER_COL, []string{"userid"})
	mongoc.EnsureIndex(SN_DB, GRAPH_FLWEE_COL, []string{"userid"})
	gsrv.mongoc = mongoc

	fsls, err := NewFsLibs(SOCIAL_NETWORK_GRAPH)
	if err != nil {
		return err
	}
	cachec, err := cachedsvcclnt.NewCachedSvcClnt(fsls, jobname)
	if err != nil {
		return err
	}
	gsrv.cachec = cachec
	ch, err := sigmarpcchan.NewSigmaRPCCh(fsls, SOCIAL_NETWORK_USER)
	if err != nil {
		return err
	}
	rpcc := rpcclnt.NewRPCClnt(ch)
	gsrv.userc = rpcc
	dbg.DPrintf(dbg.SOCIAL_NETWORK_GRAPH, "Starting graph service\n")
	perf, err := perf.NewPerf(fsls[0].ProcEnv(), perf.SOCIAL_NETWORK_GRAPH)
	if err != nil {
		dbg.DFatalf("NewPerf err %v\n", err)
	}
	defer perf.Done()
	return ssrv.RunServer()
}

func (gsrv *GraphSrv) GetFollowers(
	ctx fs.CtxI, req proto.GetFollowersRequest, res *proto.GraphGetResponse) error {
	res.Ok = "No"
	res.Userids = make([]int64, 0)
	followers, err := gsrv.getFollowers(req.Followeeid)
	if err != nil {
		return err
	}
	res.Userids = followers
	res.Ok = GRAPH_QUERY_OK
	return nil
}

func (gsrv *GraphSrv) GetFollowees(
	ctx fs.CtxI, req proto.GetFolloweesRequest, res *proto.GraphGetResponse) error {
	res.Ok = "No"
	res.Userids = make([]int64, 0)
	followees, err := gsrv.getFollowees(req.Followerid)
	if err != nil {
		return err
	}
	res.Userids = followees
	res.Ok = GRAPH_QUERY_OK
	return nil
}

func (gsrv *GraphSrv) Follow(
	ctx fs.CtxI, req proto.FollowRequest, res *proto.GraphUpdateResponse) error {
	return gsrv.updateGraph(req.Followerid, req.Followeeid, true, res)
}

func (gsrv *GraphSrv) Unfollow(
	ctx fs.CtxI, req proto.UnfollowRequest, res *proto.GraphUpdateResponse) error {
	return gsrv.updateGraph(req.Followerid, req.Followeeid, false, res)
}

func (gsrv *GraphSrv) FollowWithUname(
	ctx fs.CtxI, req proto.FollowWithUnameRequest, res *proto.GraphUpdateResponse) error {
	return gsrv.updateGraphWithUname(req.Followeruname, req.Followeeuname, true, res)
}

func (gsrv *GraphSrv) UnfollowWithUname(
	ctx fs.CtxI, req proto.UnfollowWithUnameRequest, res *proto.GraphUpdateResponse) error {
	return gsrv.updateGraphWithUname(req.Followeruname, req.Followeeuname, false, res)
}

func (gsrv *GraphSrv) updateGraph(
	followerid, followeeid int64, isFollow bool, res *proto.GraphUpdateResponse) error {
	res.Ok = "No"
	if followerid == followeeid {
		if isFollow {
			res.Ok = "Cannot follow self."
		} else {
			res.Ok = "Cannot unfollow self."
		}
		return nil
	}
	var err1, err2 error
	if isFollow {
		err1 = gsrv.mongoc.Upsert(
			SN_DB, GRAPH_FLWER_COL, bson.M{"userid": followeeid},
			bson.M{"$addToSet": bson.M{"edges": followerid}})
		err2 = gsrv.mongoc.Upsert(
			SN_DB, GRAPH_FLWEE_COL, bson.M{"userid": followerid},
			bson.M{"$addToSet": bson.M{"edges": followeeid}})
	} else {
		err1 = gsrv.mongoc.Update(
			SN_DB, GRAPH_FLWER_COL, bson.M{"userid": followeeid},
			bson.M{"$pull": bson.M{"edges": followerid}})
		err2 = gsrv.mongoc.Update(
			SN_DB, GRAPH_FLWEE_COL, bson.M{"userid": followerid},
			bson.M{"$pull": bson.M{"edges": followeeid}})
	}
	if err1 != nil || err2 != nil {
		return fmt.Errorf("error updating graph %v %v", err1, err2)
	}
	res.Ok = GRAPH_QUERY_OK
	return gsrv.clearCache(followerid, followeeid)
}

func (gsrv *GraphSrv) updateGraphWithUname(
	follwer_uname, followee_uname string, isFollow bool, res *proto.GraphUpdateResponse) error {
	res.Ok = "No"
	// get follower
	follower_arg := &proto.CheckUserRequest{Usernames: []string{follwer_uname}}
	follower_res := &proto.CheckUserResponse{}
	if err := gsrv.userc.RPC("UserSrv.CheckUser", follower_arg, follower_res); err != nil {
		return err
	} else if follower_res.Ok != USER_QUERY_OK {
		res.Ok = "Follower does not exist"
		return nil
	}
	followerid := follower_res.Userids[0]
	// get followee id
	followee_arg := &proto.CheckUserRequest{Usernames: []string{followee_uname}}
	followee_res := &proto.CheckUserResponse{}
	if err := gsrv.userc.RPC("UserSrv.CheckUser", followee_arg, followee_res); err != nil {
		return err
	} else if followee_res.Ok != USER_QUERY_OK {
		res.Ok = "Followee does not exist"
		return nil
	}
	followeeid := followee_res.Userids[0]
	return gsrv.updateGraph(followerid, followeeid, isFollow, res)
}

func (gsrv *GraphSrv) clearCache(followerid, followeeid int64) error {
	follower_key := FOLLOWER_CACHE_PREFIX + strconv.FormatInt(followeeid, 10)
	followee_key := FOLLOWEE_CACHE_PREFIX + strconv.FormatInt(followerid, 10)
	if err := gsrv.cachec.Delete(follower_key); err != nil {
		if !cache.IsMiss(err) {
			return err
		}
	}
	if err := gsrv.cachec.Delete(followee_key); err != nil {
		if !cache.IsMiss(err) {
			return err
		}
	}
	return nil
}

// Define getFollowers and getFollowees explicitly for clarity
func (gsrv *GraphSrv) getFollowers(userid int64) ([]int64, error) {
	key := FOLLOWER_CACHE_PREFIX + strconv.FormatInt(userid, 10)
	flwERInfo := &EdgeInfo{}
	cacheItem := &proto.CacheItem{}
	if err := gsrv.cachec.Get(key, cacheItem); err != nil {
		if !cache.IsMiss(err) {
			return nil, err
		}
		dbg.DPrintf(dbg.SOCIAL_NETWORK_GRAPH, "FollowER %v cache miss\n", key)
		f, err := gsrv.mongoc.FindOne(SN_DB, GRAPH_FLWER_COL, bson.M{"userid": userid}, flwERInfo)
		if err != nil {
			return nil, err
		}
		if !f {
			return make([]int64, 0), nil
		}
		encoded, _ := bson.Marshal(flwERInfo)
		gsrv.cachec.Put(key, &proto.CacheItem{Key: key, Val: encoded})
		dbg.DPrintf(dbg.SOCIAL_NETWORK_GRAPH, "Found followERs for %v in DB: %v", userid, flwERInfo)
	} else {
		bson.Unmarshal(cacheItem.Val, flwERInfo)
		dbg.DPrintf(dbg.SOCIAL_NETWORK_GRAPH, "Found followERs for %v in cache!\n", userid)
	}
	return flwERInfo.Edges, nil
}

func (gsrv *GraphSrv) getFollowees(userid int64) ([]int64, error) {
	key := FOLLOWEE_CACHE_PREFIX + strconv.FormatInt(userid, 10)
	flwEEInfo := &EdgeInfo{}
	cacheItem := &proto.CacheItem{}
	if err := gsrv.cachec.Get(key, cacheItem); err != nil {
		if !cache.IsMiss(err) {
			return nil, err
		}
		dbg.DPrintf(dbg.SOCIAL_NETWORK_GRAPH, "FollowEE %v cache miss\n", key)
		f, err := gsrv.mongoc.FindOne(SN_DB, GRAPH_FLWEE_COL, bson.M{"userid": userid}, flwEEInfo)
		if err != nil {
			return nil, err
		}
		if !f {
			return make([]int64, 0), nil
		}
		encoded, _ := bson.Marshal(flwEEInfo)
		gsrv.cachec.Put(key, &proto.CacheItem{Key: key, Val: encoded})
		dbg.DPrintf(dbg.SOCIAL_NETWORK_GRAPH, "Found followEEs for  %v in DB: %v", userid, flwEEInfo)
	} else {
		bson.Unmarshal(cacheItem.Val, flwEEInfo)
		dbg.DPrintf(dbg.SOCIAL_NETWORK_GRAPH, "Found followEEs for %v in cache!\n", userid)
	}
	return flwEEInfo.Edges, nil
}

type EdgeInfo struct {
	Userid int64   `bson:userid`
	Edges  []int64 `bson:edges`
}

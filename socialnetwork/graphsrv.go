package socialnetwork

import (
	sp "sigmaos/sigmap"
	dbg "sigmaos/debug"
	"sigmaos/protdevsrv"
	"sigmaos/protdevclnt"
	"sigmaos/dbclnt"
	"sigmaos/cacheclnt"
	"sigmaos/fs"
	"sigmaos/socialnetwork/proto"
	"strconv"
	"fmt"
)

// YH:
// Social Graph service for social network
// for now we use sql instead of MongoDB

const (
	GRAPH_QUERY_OK = "OK"
	FOLLOWER_CACHE_PREFIX = "followers_"
	FOLLOWEE_CACHE_PREFIX = "followees_"
)

type GraphSrv struct {
	dbc    *dbclnt.DbClnt
	cachec *cacheclnt.CacheClnt
	userc  *protdevclnt.ProtDevClnt
}

func RunGraphSrv(public bool, jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_GRAPH, "Creating graph service\n")
	gsrv := &GraphSrv{}
	pds, err := protdevsrv.MakeProtDevSrvPublic(sp.SOCIAL_NETWORK_GRAPH, gsrv, public)
	if err != nil {
		return err
	}
	dbc, err := dbclnt.MkDbClnt(pds.MemFs.SigmaClnt().FsLib, sp.DBD)
	if err != nil {
		return err
	}
	gsrv.dbc = dbc
	fsls := MakeFsLibs(sp.SOCIAL_NETWORK_GRAPH, pds.MemFs.SigmaClnt().FsLib)
	cachec, err := cacheclnt.MkCacheClnt(fsls, jobname)
	if err != nil {
		return err
	}
	gsrv.cachec = cachec
	pdc, err := protdevclnt.MkProtDevClnt(fsls, sp.SOCIAL_NETWORK_USER)
	if err != nil {
		return err
	}
	gsrv.userc = pdc
	dbg.DPrintf(dbg.SOCIAL_NETWORK_GRAPH, "Starting graph service\n")
	return pds.RunServer()
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
	var q string
	if isFollow {
		q = fmt.Sprintf(
			"INSERT INTO socialnetwork_graph (followerid, followeeid) VALUES ('%v', '%v');", 
			followerid, followeeid)
	} else {
		q = fmt.Sprintf(
			"DELETE FROM socialnetwork_graph WHERE followerid='%v' AND followeeid='%v';", 
			followerid, followeeid)
	}
	if err := gsrv.dbc.Exec(q); err != nil {
		return err
	}
	res.Ok = GRAPH_QUERY_OK
	return gsrv.clearCache(followerid, followeeid)
}

func (gsrv *GraphSrv) updateGraphWithUname(
		follwer_uname, followee_uname string, isFollow bool, res *proto.GraphUpdateResponse) error {
	res.Ok = "No"
	// get follower
	follower_arg := &proto.CheckUserRequest{Username: follwer_uname}
	follower_res := &proto.UserResponse{}
	if err := gsrv.userc.RPC("User.CheckUser", follower_arg, follower_res); err != nil {
		return err
	} else if follower_res.Ok != USER_QUERY_OK {
		res.Ok = "Follower does not exist"
		return nil
	}
	followerid := follower_res.Userid
	// get followee id
	followee_arg := &proto.CheckUserRequest{Username: followee_uname}
	followee_res := &proto.UserResponse{}
	if err := gsrv.userc.RPC("User.CheckUser", followee_arg, followee_res); err != nil {
		return err
	} else 	if followee_res.Ok != USER_QUERY_OK {
		res.Ok = "Followee does not exist"
		return nil
	}
	followeeid := followee_res.Userid
	return gsrv.updateGraph(followerid, followeeid, isFollow, res)
}

func (gsrv *GraphSrv) clearCache(followerid, followeeid int64) error {
	follower_key := FOLLOWER_CACHE_PREFIX + strconv.FormatInt(followeeid, 10)
	followee_key := FOLLOWEE_CACHE_PREFIX + strconv.FormatInt(followerid, 10)
	if err := gsrv.cachec.Delete(follower_key); err != nil {
		if !gsrv.cachec.IsMiss(err) {
			return err
		} 
	}	
	if err := gsrv.cachec.Delete(followee_key); err != nil {
		if !gsrv.cachec.IsMiss(err) {
			return err
		} 
	}
	return nil
}

// Define getFollowers and getFollowees explicitly for clarity
func (gsrv *GraphSrv) getFollowers(userid int64) ([]int64, error) {
	key := FOLLOWER_CACHE_PREFIX + strconv.FormatInt(userid, 10)
	followers := &proto.UseridList{}
	if err := gsrv.cachec.Get(key, followers); err != nil {
		if !gsrv.cachec.IsMiss(err) {
			return nil, err
		}
		dbg.DPrintf(dbg.SOCIAL_NETWORK_GRAPH, "FollowER %v cache miss\n", key)
		q := fmt.Sprintf("SELECT * from socialnetwork_graph where followeeid='%v';", userid)
		var edges []proto.Edge
		if err := gsrv.dbc.Query(q, &edges); err != nil {
			return nil, err
		}
		if len(edges) == 0 {
			return make([]int64, 0), nil
		}
		dbg.DPrintf(dbg.SOCIAL_NETWORK_GRAPH, "Found followERs for  %v in DB: %v\n", userid, edges)
		for _, edge := range(edges) {
			followers.Userids = append(followers.Userids, edge.Followerid)
		}
		gsrv.cachec.Put(key, followers)
	} else {
		dbg.DPrintf(dbg.SOCIAL_NETWORK_GRAPH, "Found followERs for %v in cache!\n", userid)
	}	
	return followers.Userids, nil
}

func (gsrv *GraphSrv) getFollowees(userid int64) ([]int64, error) {
	key := FOLLOWEE_CACHE_PREFIX + strconv.FormatInt(userid, 10)
	followees := &proto.UseridList{}
	if err := gsrv.cachec.Get(key, followees); err != nil {
		if !gsrv.cachec.IsMiss(err) {
			return nil, err
		}
		dbg.DPrintf(dbg.SOCIAL_NETWORK_GRAPH, "FollowEE %v cache miss\n", key)
		q := fmt.Sprintf("SELECT * from socialnetwork_graph where followerid='%v';", userid)
		var edges []proto.Edge
		if err := gsrv.dbc.Query(q, &edges); err != nil {
			return nil, err
		}
		if len(edges) == 0 {
			return make([]int64, 0), nil
		}
		dbg.DPrintf(dbg.SOCIAL_NETWORK_GRAPH, "Found followEEs for  %v in DB: %v\n", userid, edges)
		for _, edge := range(edges) {
			followees.Userids = append(followees.Userids, edge.Followeeid)
		}
		gsrv.cachec.Put(key, followees)
	} else {
		dbg.DPrintf(dbg.SOCIAL_NETWORK_GRAPH, "Found followEEs for %v in cache!\n", userid)
	}	
	return followees.Userids, nil
}

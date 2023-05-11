package socialnetwork

import (
	sp "sigmaos/sigmap"
	dbg "sigmaos/debug"
	"sigmaos/protdevsrv"
	"sigmaos/cacheclnt"
	"sigmaos/protdevclnt"
	"sigmaos/fs"
	"sigmaos/socialnetwork/proto"
	"fmt"
	"strconv"
)

// YH:
// Home timeline service for social network
// No db connection. Only use cache. 

const (
	HOME_CACHE_PREFIX = "home_"
	HOME_QUERY_OK = "OK"
)

type HomeSrv struct {
	cachec *cacheclnt.CacheClnt
	postc  *protdevclnt.ProtDevClnt
	graphc *protdevclnt.ProtDevClnt
}

func RunHomeSrv(public bool, jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_HOME, "Creating home service\n")
	hsrv := &HomeSrv{}
	pds, err := protdevsrv.MakeProtDevSrvPublic(sp.SOCIAL_NETWORK_HOME, hsrv, public)
	if err != nil {
		return err
	}
	fsls := MakeFsLibs(sp.SOCIAL_NETWORK_HOME, pds.MemFs.SigmaClnt().FsLib)
	cachec, err := cacheclnt.MkCacheClnt(fsls, jobname)
	if err != nil {
		return err
	}
	hsrv.cachec = cachec
	pdc, err := protdevclnt.MkProtDevClnt(fsls, sp.SOCIAL_NETWORK_GRAPH)
	if err != nil {
		return err
	}
	hsrv.graphc = pdc
	pdc, err = protdevclnt.MkProtDevClnt(fsls, sp.SOCIAL_NETWORK_POST)
	if err != nil {
		return err
	}
	hsrv.postc = pdc
	dbg.DPrintf(dbg.SOCIAL_NETWORK_HOME, "Starting home service\n")
	return pds.RunServer()
}

func (hsrv *HomeSrv) WriteHomeTimeline(
		ctx fs.CtxI, req proto.WriteHomeTimelineRequest, res *proto.WriteTimelineResponse) error {
	res.Ok = "No."
	otherUserIds := make(map[int64]bool, 0)
	argFollower := proto.GetFollowersRequest{Followeeid: req.Timelineitem.Userid}
	resFollower := proto.GraphGetResponse{}
	err := hsrv.graphc.RPC("Graph.GetFollowers", &argFollower, &resFollower)
	if err != nil {
		return err
	}
	for _, followerid := range resFollower.Userids {
		otherUserIds[followerid] = true
	}
	for _, mentionid := range req.Usermentionids {
		otherUserIds[mentionid] = true
	}
	dbg.DPrintf(dbg.SOCIAL_NETWORK_HOME, "Updating timeline for %v users\n", len(otherUserIds))
	missing := false
	for userid := range otherUserIds {
		hometl, err := hsrv.getHomeTimeline(userid)
		if err != nil {
			res.Ok = res.Ok + fmt.Sprintf(" Error getting home timeline for %v.", userid)	
			missing = true
			continue
		}
		hometl.Postids = append([]int64{req.Timelineitem.Postid}, hometl.Postids...)	
		hometl.Timestamps = append([]int64{req.Timelineitem.Timestamp}, hometl.Timestamps...)	
		key := HOME_CACHE_PREFIX + strconv.FormatInt(userid, 10) 
		if err := hsrv.cachec.Put(key, hometl); err != nil {
			res.Ok = res.Ok + fmt.Sprintf(" Error updating home timeline for %v.", userid)	
			missing = true
		}
	}
	if !missing {
		res.Ok = HOME_QUERY_OK
	}
	return nil 
}

func (hsrv *HomeSrv) ReadHomeTimeline(
		ctx fs.CtxI, req proto.ReadTimelineRequest, res *proto.ReadTimelineResponse) error {
	res.Ok = "No"
	timeline, err := hsrv.getHomeTimeline(req.Userid)
	if err != nil {
		return err
	}
	start, stop, nItems := req.Start, req.Stop, int32(len(timeline.Postids))
	if start >= int32(nItems) || start >= stop || stop > nItems {
		res.Ok = fmt.Sprintf("Cannot process start=%v end=%v for %v items", start, stop, nItems)
		return nil
	}	
	postids := make([]int64, stop-start)
	for i := start; i < stop; i++ {
		postids[i-start] = timeline.Postids[i]
	}
	readPostReq := proto.ReadPostsRequest{Postids: postids}
	readPostRes := proto.ReadPostsResponse{}
	if err := hsrv.postc.RPC("Post.ReadPosts", &readPostReq, &readPostRes); err != nil {
		return err 
	}
	res.Ok = readPostRes.Ok
	res.Posts = readPostRes.Posts
	return nil
}

func (hsrv *HomeSrv) getHomeTimeline(userid int64) (*proto.Timeline, error) {
	key := HOME_CACHE_PREFIX + strconv.FormatInt(userid, 10) 
	timeline := &proto.Timeline{}
	if err := hsrv.cachec.Get(key, timeline); err != nil {
		if !hsrv.cachec.IsMiss(err) {
			return nil, err
		}
	}
	return timeline, nil
}

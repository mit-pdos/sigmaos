package socialnetwork

import (
	"fmt"
	"strconv"

	"gopkg.in/mgo.v2/bson"

	"sigmaos/api/fs"
	"sigmaos/apps/cache"
	cachegrpclnt "sigmaos/apps/cache/cachegrp/clnt"
	"sigmaos/apps/socialnetwork/proto"
	dbg "sigmaos/debug"
	"sigmaos/proc"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/util/perf"
)

// YH:
// Home timeline service for social network
// No db connection. Only use cache.

const (
	HOME_CACHE_PREFIX = "home_"
	HOME_QUERY_OK     = "OK"
)

type HomeSrv struct {
	cachec *cachegrpclnt.CachedSvcClnt
	postc  *rpcclnt.RPCClnt
	graphc *rpcclnt.RPCClnt
}

func RunHomeSrv(jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_HOME, "Creating home service\n")
	hsrv := &HomeSrv{}
	ssrv, err := sigmasrv.NewSigmaSrv(SOCIAL_NETWORK_HOME, hsrv, proc.GetProcEnv())
	if err != nil {
		return err
	}
	fsl, err := NewFsLib(SOCIAL_NETWORK_HOME, ssrv.MemFs.SigmaClnt().GetDialProxyClnt())
	if err != nil {
		return err
	}
	hsrv.cachec = cachegrpclnt.NewCachedSvcClnt(fsl, jobname)
	rpcc, err := sprpcclnt.NewRPCClnt(fsl, SOCIAL_NETWORK_GRAPH)
	if err != nil {
		return err
	}
	hsrv.graphc = rpcc
	rpcc, err = sprpcclnt.NewRPCClnt(fsl, SOCIAL_NETWORK_POST)
	if err != nil {
		return err
	}
	hsrv.postc = rpcc
	dbg.DPrintf(dbg.SOCIAL_NETWORK_HOME, "Starting home service\n")
	perf, err := perf.NewPerf(fsl.ProcEnv(), perf.SOCIAL_NETWORK_HOME)
	if err != nil {
		dbg.DFatalf("NewPerf err %v\n", err)
	}
	defer perf.Done()

	return ssrv.RunServer()
}

func (hsrv *HomeSrv) WriteHomeTimeline(
	ctx fs.CtxI, req proto.WriteHomeTimelineReq, res *proto.WriteTimelineRep) error {
	res.Ok = "No."
	otherUserIds := make(map[int64]bool, 0)
	argFollower := proto.GetFollowersReq{Followeeid: req.Userid}
	resFollower := proto.GraphGetRep{}
	err := hsrv.graphc.RPC("GraphSrv.GetFollowers", &argFollower, &resFollower)
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
		hometl.Postids = append(hometl.Postids, req.Postid)
		hometl.Timestamps = append(hometl.Timestamps, req.Timestamp)
		key := HOME_CACHE_PREFIX + strconv.FormatInt(userid, 10)
		encoded, _ := bson.Marshal(hometl)
		hsrv.cachec.Put(key, &proto.CacheItem{Key: key, Val: encoded})
	}
	if !missing {
		res.Ok = HOME_QUERY_OK
	}
	return nil
}

func (hsrv *HomeSrv) ReadHomeTimeline(
	ctx fs.CtxI, req proto.ReadTimelineReq, res *proto.ReadTimelineRep) error {
	res.Ok = "No"
	timeline, err := hsrv.getHomeTimeline(req.Userid)
	if err != nil {
		return err
	}
	start, stop, nItems := req.Start, req.Stop, int32(len(timeline.Postids))
	if start >= int32(nItems) || start >= stop {
		res.Ok = fmt.Sprintf("Cannot process start=%v end=%v for %v items", start, stop, nItems)
		return nil
	}
	if stop > nItems {
		stop = nItems
	}
	postids := make([]int64, stop-start)
	for i := start; i < stop; i++ {
		postids[i-start] = timeline.Postids[nItems-i-1]
	}
	readPostReq := proto.ReadPostsReq{Postids: postids}
	readPostRes := proto.ReadPostsRep{}
	if err := hsrv.postc.RPC("PostSrv.ReadPosts", &readPostReq, &readPostRes); err != nil {
		return err
	}
	res.Ok = readPostRes.Ok
	res.Posts = readPostRes.Posts
	return nil
}

func (hsrv *HomeSrv) getHomeTimeline(userid int64) (*Timeline, error) {
	key := HOME_CACHE_PREFIX + strconv.FormatInt(userid, 10)
	timeline := &Timeline{}
	cacheItem := &proto.CacheItem{}
	if err := hsrv.cachec.Get(key, cacheItem); err != nil {
		if !cache.IsMiss(err) {
			return nil, err
		}
		timeline.Userid = userid
	} else {
		bson.Unmarshal(cacheItem.Val, timeline)
		dbg.DPrintf(dbg.SOCIAL_NETWORK_HOME, "Found home timeline %v in cache! %v", userid, timeline)
	}
	return timeline, nil
}

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
	mongoclnt "sigmaos/proxy/mongo/clnt"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/util/perf"
)

// YH:
// Timeline service for social network
// for now we use sql instead of MongoDB

const (
	TIMELINE_QUERY_OK     = "OK"
	TIMELINE_CACHE_PREFIX = "timeline_"
)

type TimelineSrv struct {
	mongoc *mongoclnt.MongoClnt
	cachec *cachegrpclnt.CachedSvcClnt
	postc  *rpcclnt.RPCClnt
}

func RunTimelineSrv(jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_TIMELINE, "Creating timeline service\n")
	tlsrv := &TimelineSrv{}
	ssrv, err := sigmasrv.NewSigmaSrv(SOCIAL_NETWORK_TIMELINE, tlsrv, proc.GetProcEnv())
	if err != nil {
		return err
	}
	mongoc, err := mongoclnt.NewMongoClnt(ssrv.MemFs.SigmaClnt().FsLib)
	if err != nil {
		return err
	}
	mongoc.EnsureIndex(SN_DB, TIMELINE_COL, []string{"userid"})
	tlsrv.mongoc = mongoc
	fsl, err := NewFsLib(SOCIAL_NETWORK_TIMELINE, ssrv.MemFs.SigmaClnt().GetDialProxyClnt())
	if err != nil {
		return err
	}
	tlsrv.cachec = cachegrpclnt.NewCachedSvcClnt(fsl, jobname)
	rpcc, err := sprpcclnt.NewRPCClnt(fsl, SOCIAL_NETWORK_POST)
	if err != nil {
		return err
	}
	tlsrv.postc = rpcc
	dbg.DPrintf(dbg.SOCIAL_NETWORK_TIMELINE, "Starting timeline service\n")
	perf, err := perf.NewPerf(fsl.ProcEnv(), perf.SOCIAL_NETWORK_TIMELINE)
	if err != nil {
		dbg.DFatalf("NewPerf err %v\n", err)
	}
	defer perf.Done()

	return ssrv.RunServer()
}

func (tlsrv *TimelineSrv) WriteTimeline(
	ctx fs.CtxI, req proto.WriteTimelineReq, res *proto.WriteTimelineRep) error {
	res.Ok = "No"
	err := tlsrv.mongoc.Upsert(
		SN_DB, TIMELINE_COL, bson.M{"userid": req.Userid},
		bson.M{"$push": bson.M{"postids": req.Postid, "timestamps": req.Timestamp}})
	if err != nil {
		return err
	}
	res.Ok = TIMELINE_QUERY_OK
	key := TIMELINE_CACHE_PREFIX + strconv.FormatInt(req.Userid, 10)
	if err := tlsrv.cachec.Delete(key); err != nil {
		if !cache.IsMiss(err) {
			return err
		}
	}
	return nil
}

func (tlsrv *TimelineSrv) ReadTimeline(
	ctx fs.CtxI, req proto.ReadTimelineReq, res *proto.ReadTimelineRep) error {
	res.Ok = "No"
	timeline, err := tlsrv.getUserTimeline(req.Userid)
	if err != nil {
		return err
	}
	if timeline == nil {
		res.Ok = "No timeline item"
		return nil
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
	if err := tlsrv.postc.RPC("PostSrv.ReadPosts", &readPostReq, &readPostRes); err != nil {
		return err
	}
	res.Ok = readPostRes.Ok
	res.Posts = readPostRes.Posts
	return nil
}

func (tlsrv *TimelineSrv) getUserTimeline(userid int64) (*Timeline, error) {
	key := TIMELINE_CACHE_PREFIX + strconv.FormatInt(userid, 10)
	timeline := &Timeline{}
	cacheItem := &proto.CacheItem{}
	if err := tlsrv.cachec.Get(key, cacheItem); err != nil {
		if !cache.IsMiss(err) {
			return nil, err
		}
		dbg.DPrintf(dbg.SOCIAL_NETWORK_TIMELINE, "Timeline %v cache miss\n", key)
		found, err := tlsrv.mongoc.FindOne(SN_DB, TIMELINE_COL, bson.M{"userid": userid}, timeline)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, nil
		}
		encoded, _ := bson.Marshal(timeline)
		dbg.DPrintf(dbg.SOCIAL_NETWORK_TIMELINE, "Found timeline %v in DB: %v", userid, timeline)
		tlsrv.cachec.Put(key, &proto.CacheItem{Key: key, Val: encoded})
	} else {
		dbg.DPrintf(dbg.SOCIAL_NETWORK_TIMELINE, "Found timeline %v in cache!\n", userid)
		bson.Unmarshal(cacheItem.Val, timeline)
	}
	return timeline, nil
}

type Timeline struct {
	Userid     int64   `bson:userid`
	Postids    []int64 `bson:postids`
	Timestamps []int64 `bson:timestamps`
}

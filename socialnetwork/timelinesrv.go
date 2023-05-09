package socialnetwork

import (
	sp "sigmaos/sigmap"
	dbg "sigmaos/debug"
	"sigmaos/protdevsrv"
	"sigmaos/dbclnt"
	"sigmaos/cacheclnt"
	"sigmaos/protdevclnt"
	"sigmaos/fs"
	"sigmaos/socialnetwork/proto"
	"strconv"
	"fmt"
	"sort"
)

// YH:
// Timeline service for social network
// for now we use sql instead of MongoDB

const (
	TIMELINE_QUERY_OK = "OK"
	TIMELINE_CACHE_PREFIX = "timeline_"
)

type TimelineSrv struct {
	dbc    *dbclnt.DbClnt
	cachec *cacheclnt.CacheClnt
	postc  *protdevclnt.ProtDevClnt
}

func RunTimelineSrv(public bool, jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_TIMELINE, "Creating timeline service\n")
	tlsrv := &TimelineSrv{}
	pds, err := protdevsrv.MakeProtDevSrvPublic(sp.SOCIAL_NETWORK_TIMELINE, tlsrv, public)
	if err != nil {
		return err
	}
	dbc, err := dbclnt.MkDbClnt(pds.MemFs.SigmaClnt().FsLib, sp.DBD)
	if err != nil {
		return err
	}
	tlsrv.dbc = dbc
	cachec, err := cacheclnt.MkCacheClnt(pds.MemFs.SigmaClnt().FsLib, jobname)
	if err != nil {
		return err
	}
	tlsrv.cachec = cachec
	pdc, err := protdevclnt.MkProtDevClnt(pds.SigmaClnt().FsLib, sp.SOCIAL_NETWORK_POST)
	if err != nil {
		return err
	}
	tlsrv.postc = pdc
	dbg.DPrintf(dbg.SOCIAL_NETWORK_TIMELINE, "Starting timeline service\n")
	return pds.RunServer()
}

func (tlsrv *TimelineSrv) WriteTimeline(
		ctx fs.CtxI, req proto.WriteTimelineRequest, res *proto.WriteTimelineResponse) error {
	res.Ok = "No"
	item := req.Timelineitem
	q := fmt.Sprintf(
		"INSERT INTO socialnetwork_timeline (userid, postid, timestamp) VALUES ('%v', '%v', '%v')", 
		item.Userid, item.Postid, item.Timestamp)
	if err := tlsrv.dbc.Exec(q); err != nil {
		return nil
	}
	res.Ok = TIMELINE_QUERY_OK
	key := TIMELINE_CACHE_PREFIX + strconv.FormatInt(item.Userid, 10)
	if err := tlsrv.cachec.Delete(key); err != nil {
		if !tlsrv.cachec.IsMiss(err) {
			return err
		} 
	}
	return nil 
}

func (tlsrv *TimelineSrv) ReadTimeline(
		ctx fs.CtxI, req proto.ReadTimelineRequest, res *proto.ReadTimelineResponse) error {
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
	if err := tlsrv.postc.RPC("Post.ReadPosts", &readPostReq, &readPostRes); err != nil {
		return err 
	}
	res.Ok = readPostRes.Ok
	res.Posts = readPostRes.Posts
	return nil
}

func (tlsrv *TimelineSrv) getUserTimeline(userid int64) (*proto.Timeline, error) {
	key := TIMELINE_CACHE_PREFIX + strconv.FormatInt(userid, 10) 
	timeline := &proto.Timeline{}
	if err := tlsrv.cachec.Get(key, timeline); err != nil {
		if !tlsrv.cachec.IsMiss(err) {
			return nil, err
		}
		dbg.DPrintf(dbg.SOCIAL_NETWORK_TIMELINE, "Timeline %v cache miss\n", key)
		q := fmt.Sprintf("SELECT * from socialnetwork_timeline where userid='%v';", userid)
		var timelineItems []proto.TimelineItem
		if err := tlsrv.dbc.Query(q, &timelineItems); err != nil {
			return nil, err
		}
		nItems := len(timelineItems)
		if nItems == 0 {
			return nil, nil
		}
		dbg.DPrintf(dbg.SOCIAL_NETWORK_TIMELINE, "Found %v items for %v in DB.", nItems, userid)
		// sort timeline items in reverse order
		sort.Slice(timelineItems, func(i, j int) bool {
			return timelineItems[i].Timestamp > timelineItems[j].Timestamp})
		dbg.DPrintf(dbg.SOCIAL_NETWORK_TIMELINE, "sorted items: %v\n", timelineItems)
		timeline.Userid = userid
		timeline.Postids = make([]int64, nItems)
		timeline.Timestamps = make([]int64, nItems)
		for i, item := range(timelineItems) {
			timeline.Postids[i], timeline.Timestamps[i] = item.Postid, item.Timestamp
		}
		tlsrv.cachec.Put(key, timeline)
	} else {
		dbg.DPrintf(dbg.SOCIAL_NETWORK_TIMELINE, "Found timeline %v in cache!\n", userid)
	}
	return timeline, nil
}

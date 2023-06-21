package socialnetwork

import (
	sp "sigmaos/sigmap"
	dbg "sigmaos/debug"
	"sigmaos/perf"
	"sigmaos/protdevsrv"
	"sigmaos/protdevclnt"
	"sigmaos/fs"
	"sigmaos/socialnetwork/proto"
	"math/rand"
	"sync"
	"time"
	"fmt"
)

// YH:
// Compose post service for social network
// No db or cache connection. 

const (
	COMPOSE_QUERY_OK = "OK"
)

type ComposeSrv struct {
	textc  *protdevclnt.ProtDevClnt
	postc  *protdevclnt.ProtDevClnt
	tlc    *protdevclnt.ProtDevClnt
	homec  *protdevclnt.ProtDevClnt
	sid    int32
	pcount int32
	mu     sync.Mutex
}

func RunComposeSrv(public bool, jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_COMPOSE, "Creating compose service\n")
	csrv := &ComposeSrv{}
	csrv.sid = rand.Int31n(536870912) // 2^29
	pds, err := protdevsrv.MakeProtDevSrvPublic(sp.SOCIAL_NETWORK_COMPOSE, csrv, public)
	if err != nil {
		return err
	}
	fsls := MakeFsLibs(sp.SOCIAL_NETWORK_POST)
	pdc, err := protdevclnt.MkProtDevClnt(fsls, sp.SOCIAL_NETWORK_TEXT)
	if err != nil {
		return err
	}
	csrv.textc = pdc	
	pdc, err = protdevclnt.MkProtDevClnt(fsls, sp.SOCIAL_NETWORK_POST)
	if err != nil {
		return err
	}
	csrv.postc = pdc	
	pdc, err = protdevclnt.MkProtDevClnt(fsls, sp.SOCIAL_NETWORK_TIMELINE)
	if err != nil {
		return err
	}
	csrv.tlc = pdc	
	pdc, err = protdevclnt.MkProtDevClnt(fsls, sp.SOCIAL_NETWORK_HOME)
	if err != nil {
		return err
	}
	csrv.homec = pdc	
	dbg.DPrintf(dbg.SOCIAL_NETWORK_COMPOSE, "Starting compose service %v\n", csrv.sid)
	perf, err := perf.MakePerf(perf.SOCIAL_NETWORK_COMPOSE)
	if err != nil {
		dbg.DFatalf("MakePerf err %v\n", err)
	}
	defer perf.Done()
	return pds.RunServer()
}

func (csrv *ComposeSrv) ComposePost(
		ctx fs.CtxI, req proto.ComposePostRequest, res *proto.ComposePostResponse) error {
	res.Ok = "No"
	timestamp := time.Now().UnixNano()
	if req.Text == "" {
		res.Ok = "Cannot compose empty post!"
		return nil
	}
	// process text
	textReq := proto.ProcessTextRequest{Text: req.Text}
	textRes := proto.ProcessTextResponse{}
	if err := csrv.textc.RPC("Text.ProcessText", &textReq, &textRes); err != nil {
		return err
	}
	if textRes.Ok != TEXT_QUERY_OK {
		res.Ok += " Text Error: " + textRes.Ok
		return nil
	} 
	// create post
	post := &proto.Post{}
	post.Postid = csrv.getNextPostId()
	post.Posttype = req.Posttype
	post.Timestamp = timestamp
	post.Creator = &proto.UserRef{Userid: req.Userid, Username: req.Username}
	post.Text = textRes.Text
	post.Usermentions = textRes.Usermentions
	post.Urls = textRes.Urls
	for idx, mid := range req.Mediaids {
		post.Medias = append(
			post.Medias, &proto.MediaRef{Mediaid: mid, Mediatype: req.Mediatypes[idx]})
	}
	dbg.DPrintf(dbg.SOCIAL_NETWORK_COMPOSE, "composing post: %v\n", post)
	
	// concurrently add post to storage and timelines

	var wg sync.WaitGroup
	var postErr, tlErr, homeErr error
	postReq := proto.StorePostRequest{Post: post}
	postRes := proto.StorePostResponse{}
	tlReq := proto.WriteTimelineRequest{
		Timelineitem: &proto.TimelineItem{
			Userid: post.Creator.Userid, Postid: post.Postid, Timestamp: post.Timestamp}}
	tlRes := proto.WriteTimelineResponse{}
	mentionids := make([]int64, len(post.Usermentions))
	for idx, mention := range post.Usermentions {
		mentionids[idx] = mention.Userid
	}
	homeReq := proto.WriteHomeTimelineRequest{
		Usermentionids: mentionids,
		Timelineitem: &proto.TimelineItem{
			Userid: post.Creator.Userid, Postid: post.Postid, Timestamp: post.Timestamp}}
	homeRes := proto.WriteTimelineResponse{}
	wg.Add(3)
	go func() {
		defer wg.Done()
		postErr = csrv.postc.RPC("Post.StorePost", &postReq, &postRes)
	}()
	go func() {
		defer wg.Done()
		tlErr = csrv.tlc.RPC("Timeline.WriteTimeline", &tlReq, &tlRes) 
	}()
	go func() {
		defer wg.Done()
		homeErr = csrv.homec.RPC("Home.WriteHomeTimeline", &homeReq, &homeRes)
	}()
	wg.Wait()
	if postErr != nil || tlErr != nil || homeErr != nil {
		return fmt.Errorf("%w; %w; %w", postErr, tlErr, homeErr)
	}
	if postRes.Ok != POST_QUERY_OK {
		res.Ok += " Post Error: " + postRes.Ok
		return nil
	} 
	if tlRes.Ok != TIMELINE_QUERY_OK {
		res.Ok += " Timeline Error: " + tlRes.Ok
		return nil
	}
	if homeRes.Ok != HOME_QUERY_OK {
		res.Ok += " Home Error: " + homeRes.Ok
		return nil
	}
	res.Ok = COMPOSE_QUERY_OK
	return nil
}

func (csrv *ComposeSrv) incCountSafe() int32 {
	csrv.mu.Lock()
	defer csrv.mu.Unlock()
	csrv.pcount++
	return csrv.pcount
}

func (csrv *ComposeSrv) getNextPostId() int64 {
	return int64(csrv.sid)*1e10 + int64(csrv.incCountSafe())
}


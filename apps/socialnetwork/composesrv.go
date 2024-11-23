package socialnetwork

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"sigmaos/apps/socialnetwork/proto"
	dbg "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/proc"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/util/perf"
)

// YH:
// Compose post service for social network
// No db or cache connection.

const (
	COMPOSE_QUERY_OK = "OK"
)

type ComposeSrv struct {
	textc  *rpcclnt.RPCClnt
	postc  *rpcclnt.RPCClnt
	tlc    *rpcclnt.RPCClnt
	homec  *rpcclnt.RPCClnt
	sid    int32
	pcount int32
	mu     sync.Mutex
}

func RunComposeSrv(jobname string) error {
	dbg.DPrintf(dbg.SOCIAL_NETWORK_COMPOSE, "Creating compose service\n")
	csrv := &ComposeSrv{}
	csrv.sid = rand.Int31n(536870912) // 2^29

	ssrv, err := sigmasrv.NewSigmaSrv(SOCIAL_NETWORK_COMPOSE, csrv, proc.GetProcEnv())
	if err != nil {
		return err
	}
	fsl, err := NewFsLib(SOCIAL_NETWORK_POST, ssrv.MemFs.SigmaClnt().GetDialProxyClnt())
	if err != nil {
		return err
	}
	rpcc, err := sprpcclnt.NewRPCClnt(fsl, SOCIAL_NETWORK_TEXT)
	if err != nil {
		return err
	}
	csrv.textc = rpcc
	rpcc, err = sprpcclnt.NewRPCClnt(fsl, SOCIAL_NETWORK_POST)
	if err != nil {
		return err
	}
	csrv.postc = rpcc
	rpcc, err = sprpcclnt.NewRPCClnt(fsl, SOCIAL_NETWORK_TIMELINE)
	if err != nil {
		return err
	}
	csrv.tlc = rpcc
	rpcc, err = sprpcclnt.NewRPCClnt(fsl, SOCIAL_NETWORK_HOME)
	if err != nil {
		return err
	}
	csrv.homec = rpcc
	dbg.DPrintf(dbg.SOCIAL_NETWORK_COMPOSE, "Starting compose service %v\n", csrv.sid)
	perf, err := perf.NewPerf(fsl.ProcEnv(), perf.SOCIAL_NETWORK_COMPOSE)
	if err != nil {
		dbg.DFatalf("NewPerf err %v\n", err)
	}
	defer perf.Done()
	return ssrv.RunServer()
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
	if err := csrv.textc.RPC("TextSrv.ProcessText", &textReq, &textRes); err != nil {
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
	post.Creator = req.Userid
	post.Creatoruname = req.Username
	post.Text = textRes.Text
	post.Usermentions = textRes.Usermentions
	post.Urls = textRes.Urls
	post.Medias = req.Mediaids
	dbg.DPrintf(dbg.SOCIAL_NETWORK_COMPOSE, "composing post: %v\n", post)

	// concurrently add post to storage and timelines

	var wg sync.WaitGroup
	var postErr, tlErr, homeErr error
	postReq := proto.StorePostRequest{Post: post}
	postRes := proto.StorePostResponse{}
	tlReq := proto.WriteTimelineRequest{
		Userid: req.Userid, Postid: post.Postid, Timestamp: post.Timestamp}
	tlRes := proto.WriteTimelineResponse{}
	homeReq := proto.WriteHomeTimelineRequest{
		Usermentionids: post.Usermentions, Userid: req.Userid,
		Postid: post.Postid, Timestamp: post.Timestamp}
	homeRes := proto.WriteTimelineResponse{}
	wg.Add(3)
	go func() {
		defer wg.Done()
		postErr = csrv.postc.RPC("PostSrv.StorePost", &postReq, &postRes)
	}()
	go func() {
		defer wg.Done()
		tlErr = csrv.tlc.RPC("TimelineSrv.WriteTimeline", &tlReq, &tlRes)
	}()
	go func() {
		defer wg.Done()
		homeErr = csrv.homec.RPC("HomeSrv.WriteHomeTimeline", &homeReq, &homeRes)
	}()
	wg.Wait()
	if postErr != nil || tlErr != nil || homeErr != nil {
		return fmt.Errorf("%v; %v; %v", postErr, tlErr, homeErr)
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

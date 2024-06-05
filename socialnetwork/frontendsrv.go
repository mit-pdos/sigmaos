package socialnetwork

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	dbg "sigmaos/debug"
	"sigmaos/perf"
	"sigmaos/port"
	"sigmaos/proc"
	"sigmaos/rpcclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmarpcchan"
	"sigmaos/socialnetwork/proto"
	"sigmaos/tracing"
)

type FrontEnd struct {
	*sigmaclnt.SigmaClnt
	p        *perf.Perf
	record   bool
	job      string
	tracer   *tracing.Tracer
	userc    *rpcclnt.RPCClnt
	graphc   *rpcclnt.RPCClnt
	tlc      *rpcclnt.RPCClnt
	homec    *rpcclnt.RPCClnt
	composec *rpcclnt.RPCClnt
}

const SERVER_NAME = "socialnetwork-frontend"

var (
	posttypesMap = map[string]proto.POST_TYPE{
		"unknown": proto.POST_TYPE_UNKNOWN,
		"post":    proto.POST_TYPE_POST,
		"repost":  proto.POST_TYPE_REPOST,
		"reply":   proto.POST_TYPE_REPLY,
		"dm":      proto.POST_TYPE_DM,
	}
)

// Run starts the server
func RunFrontendSrv(public bool, job string) error {
	frontend := &FrontEnd{}
	frontend.job = job
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return err
	}
	frontend.SigmaClnt = sc
	fsls, err := NewFsLibs(SERVER_NAME, sc.GetNetProxyClnt())
	if err != nil {
		return err
	}
	rpcc, err := sigmarpcchan.NewSigmaRPCClnt(fsls, SOCIAL_NETWORK_USER)
	if err != nil {
		return err
	}
	frontend.userc = rpcc
	rpcc, err = sigmarpcchan.NewSigmaRPCClnt(fsls, SOCIAL_NETWORK_GRAPH)
	if err != nil {
		return err
	}
	frontend.graphc = rpcc
	rpcc, err = sigmarpcchan.NewSigmaRPCClnt(fsls, SOCIAL_NETWORK_TIMELINE)
	if err != nil {
		return err
	}
	frontend.tlc = rpcc
	rpcc, err = sigmarpcchan.NewSigmaRPCClnt(fsls, SOCIAL_NETWORK_HOME)
	if err != nil {
		return err
	}
	frontend.homec = rpcc
	rpcc, err = sigmarpcchan.NewSigmaRPCClnt(fsls, SOCIAL_NETWORK_COMPOSE)
	if err != nil {
		return err
	}
	frontend.composec = rpcc
	//	frontend.tracer = tracing.Init("frontend", proc.GetSigmaJaegerIP())

	var mux *http.ServeMux
	//	var tmux *tracing.TracedHTTPMux
	//	if TRACING {
	//		tmux = tracing.NewHTTPMux()
	//		tmux.HandleFunc("/user", frontend.userHandler)
	//		tmux.HandleFunc("/hotels", frontend.searchHandler)
	//		tmux.HandleFunc("/recommendations", frontend.recommendHandler)
	//		tmux.HandleFunc("/reservation", frontend.reservationHandler)
	//		tmux.HandleFunc("/geo", frontend.geoHandler)
	//	} else {
	mux = http.NewServeMux()
	mux.HandleFunc("/user", frontend.userHandler)
	mux.HandleFunc("/compose", frontend.composeHandler)
	mux.HandleFunc("/timeline", frontend.timelineHandler)
	mux.HandleFunc("/home", frontend.homeHandler)
	mux.HandleFunc("/startrecording", frontend.startRecordingHandler)
	//	}
	dbg.DPrintf(dbg.ALWAYS, "SN public? %v", public)
	if public {
		ep, l, err := sc.GetNetProxyClnt().Listen(sp.EXTERNAL_EP, sp.NewTaddrRealm(sp.NO_IP, sp.INNER_CONTAINER_IP, port.PUBLIC_PORT))
		if err != nil {
			dbg.DFatalf("Error %v Listen: %v", public, err)
		}
		dbg.DPrintf(dbg.ALWAYS, "SN Got ep %v", ep)

		//		if TRACING {
		//			go tmux.Serve(l)
		//		} else {
		go http.Serve(l, mux)
		//		}
		if err = port.AdvertisePublicHTTPPort(frontend.FsLib, JobHTTPAddrsPath(job), ep); err != nil {
			dbg.DFatalf("AdvertisePort %v", err)
		}
	} else {
		ep, l, err := sc.GetNetProxyClnt().Listen(sp.EXTERNAL_EP, sp.NewTaddrRealm(sp.NO_IP, sp.INNER_CONTAINER_IP, sp.NO_PORT))
		if err != nil {
			dbg.DFatalf("Error %v Listen: %v", public, err)
		}
		//		if TRACING {
		//			go tmux.Serve(l)
		//		} else {
		go http.Serve(l, mux)
		dbg.DPrintf(dbg.ALWAYS, "SN advertise %v", ep)
		if err = sc.MkEndpointFile(JobHTTPAddrsPath(job), ep); err != nil {
			dbg.DFatalf("MkEndpointFile %v", err)
		}
	}

	perf, err := perf.NewPerf(frontend.ProcEnv(), perf.SOCIAL_NETWORK_FRONTEND)
	if err != nil {
		dbg.DFatalf("NewPerf err %v\n", err)
	}
	frontend.p = perf

	if err := frontend.Started(); err != nil {
		return err
	}

	return frontend.done()
}

func (s *FrontEnd) done() error {
	if err := s.WaitEvict(s.ProcEnv().GetPID()); err != nil {
		return err
	}
	dbg.DPrintf(dbg.HOTEL_WWW_STATS, "\nUserc %v", s.userc.StatsClnt())
	//	s.tracer.Flush()
	s.p.Done()
	s.ClntExit(proc.NewStatus(proc.StatusEvicted))
	return nil
}

func (s *FrontEnd) userHandler(w http.ResponseWriter, r *http.Request) {
	if s.record {
		defer s.p.TptTick(1.0)
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	rawQuery, _ := url.QueryUnescape(r.URL.RawQuery)
	urlQuery, _ := url.ParseQuery(rawQuery)
	dbg.DPrintf(dbg.SOCIAL_NETWORK_FRONTEND, "user request %v\n", rawQuery)
	//	var span trace.Span
	//	var sctx *tproto.SpanContextConfig
	//	if TRACING {
	//		_, span = s.tracer.StartContextSpan(r.Context(), "User")
	//		defer span.End()
	//		sctx = tracing.SpanToContext(span)
	//	}

	username, password := urlQuery.Get("username"), urlQuery.Get("password")
	if username == "" || password == "" {
		http.Error(w, "Please specify username and password", http.StatusBadRequest)
		return
	}
	var res proto.UserResponse
	// Check username and password
	err := s.userc.RPC("UserSrv.Login", &proto.LoginRequest{
		Username: username,
		Password: password,
	}, &res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	str := "Login successfully!"
	if res.Ok != USER_QUERY_OK {
		str = "Failed. Please check your username and password. "
	}
	reply := map[string]interface{}{
		"message": str,
	}
	json.NewEncoder(w).Encode(reply)
}

func (s *FrontEnd) composeHandler(w http.ResponseWriter, r *http.Request) {
	if s.record {
		defer s.p.TptTick(1.0)
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	rawQuery, _ := url.QueryUnescape(r.URL.RawQuery)
	urlQuery, _ := url.ParseQuery(rawQuery)
	dbg.DPrintf(dbg.SOCIAL_NETWORK_FRONTEND, "Compose request: %v\n", urlQuery)
	username, useridstr := urlQuery.Get("username"), urlQuery.Get("userid")
	var userid int64
	if useridstr == "" {
		if username == "" {
			http.Error(w, "Please specify username or id", http.StatusBadRequest)
			return
		}
		var res proto.CheckUserResponse
		// retrieve userid
		err := s.userc.RPC("UserSrv.CheckUser",
			&proto.CheckUserRequest{Usernames: []string{username}}, &res)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if res.Ok != USER_QUERY_OK {
			http.Error(w, "bad user name or id", http.StatusBadRequest)
			return
		}
		userid = res.Userids[0]
	} else {
		var err error
		userid, err = strconv.ParseInt(useridstr, 10, 64)
		if err != nil {
			http.Error(w, "bad user id format", http.StatusBadRequest)
			return
		}
	}
	// compose a post
	text, posttype, mediastr := urlQuery.Get("text"), urlQuery.Get("posttype"), urlQuery.Get("media")
	mediaids := make([]int64, 0)
	if mediastr != "" {
		for _, idstr := range strings.Split(mediastr, ",") {
			mediaid, err := strconv.ParseInt(idstr, 10, 64)
			if err != nil {
				dbg.DPrintf(dbg.SOCIAL_NETWORK_FRONTEND, "Cannot parse media: %v", idstr)
			} else {
				mediaids = append(mediaids, mediaid)
			}
		}
	}
	var res proto.ComposePostResponse
	err := s.composec.RPC("ComposeSrv.ComposePost", &proto.ComposePostRequest{
		Username: username,
		Userid:   userid,
		Text:     text,
		Posttype: parsePostTypeString(posttype),
		Mediaids: mediaids,
	}, &res)
	if err != nil {
		dbg.DPrintf(dbg.SOCIAL_NETWORK_FRONTEND, "Compose error for: %v. Err %v", text, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	str := "Compose successfully!"
	if res.Ok != COMPOSE_QUERY_OK {
		str = res.Ok
	}
	reply := map[string]interface{}{"message": str}
	json.NewEncoder(w).Encode(reply)
	dbg.DPrintf(dbg.SOCIAL_NETWORK_FRONTEND, "Completed: %v", text)
}

func (s *FrontEnd) timelineHandler(w http.ResponseWriter, r *http.Request) {
	s.timelineHandlerInner(w, r, false)
}

func (s *FrontEnd) homeHandler(w http.ResponseWriter, r *http.Request) {
	s.timelineHandlerInner(w, r, true)
}

func (s *FrontEnd) timelineHandlerInner(w http.ResponseWriter, r *http.Request, isHome bool) {
	if s.record {
		defer s.p.TptTick(1.0)
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	rawQuery, _ := url.QueryUnescape(r.URL.RawQuery)
	urlQuery, _ := url.ParseQuery(rawQuery)
	debugInfo := "Timeline request"
	if isHome {
		debugInfo = "Home timeline request"
	}
	dbg.DPrintf(dbg.SOCIAL_NETWORK_FRONTEND, "%s: %v\n", debugInfo, urlQuery)
	useridstr, startstr, stopstr :=
		urlQuery.Get("userid"), urlQuery.Get("start"), urlQuery.Get("stop")
	var err, err1, err2, err3 error
	var start, stop int64
	userid, err1 := strconv.ParseInt(useridstr, 10, 64)
	if startstr == "" {
		start = 0
	} else {
		start, err2 = strconv.ParseInt(startstr, 10, 32)
	}
	if stopstr == "" {
		stop = 1
	} else {
		stop, err2 = strconv.ParseInt(stopstr, 10, 32)
	}
	if err1 != nil || err2 != nil || err3 != nil {
		http.Error(w, "bad number format in request", http.StatusBadRequest)
		return
	}
	var res proto.ReadTimelineResponse
	if isHome {
		err = s.homec.RPC("HomeSrv.ReadHomeTimeline", &proto.ReadTimelineRequest{
			Userid: userid, Start: int32(start), Stop: int32(stop)}, &res)
	} else {
		err = s.tlc.RPC("TimelineSrv.ReadTimeline", &proto.ReadTimelineRequest{
			Userid: userid, Start: int32(start), Stop: int32(stop)}, &res)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	str := "Timeline successfully!"
	postCreators := ""
	postTimes := ""
	postContents := ""
	if res.Ok != COMPOSE_QUERY_OK {
		str = "Timeline Failed!" + res.Ok
	} else {
		for _, post := range res.Posts {
			postTimes += time.Unix(0, post.Timestamp).Format(time.UnixDate) + "; "
			postCreators += post.Creatoruname + "; "
			postContents += post.Text + "; "
		}
	}
	reply := map[string]interface{}{
		"message": str, "times": postTimes, "contents": postContents, "creators": postCreators}
	json.NewEncoder(w).Encode(reply)
}

func (s *FrontEnd) startRecordingHandler(w http.ResponseWriter, r *http.Request) {
	//	var span trace.Span
	//	var sctx *tproto.SpanContextConfig
	//	if TRACING {
	//		_, span = s.tracer.StartContextSpan(r.Context(), "Geo")
	//		defer span.End()
	//		sctx = tracing.SpanToContext(span)
	//	}
	s.record = true
	w.Header().Set("Access-Control-Allow-Origin", "*")
	dbg.DPrintf(dbg.SOCIAL_NETWORK_FRONTEND, "Start recording")
	str := "Started recording!"
	reply := map[string]interface{}{
		"message": str,
	}
	json.NewEncoder(w).Encode(reply)
}

func parsePostTypeString(str string) proto.POST_TYPE {
	c, ok := posttypesMap[strings.ToLower(str)]
	if !ok {
		c = proto.POST_TYPE_UNKNOWN
	}
	return c
}

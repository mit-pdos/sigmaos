package hotel

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	//	"context"
	//	"go.opentelemetry.io/otel/trace"
	//	tproto "sigmaos/util/tracing/proto"

	"sigmaos/apps/hotel/proto"
	db "sigmaos/debug"
	"sigmaos/proc"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	shardedsvcrpcclnt "sigmaos/rpc/shardedsvc/clnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
	"sigmaos/util/tracing"
)

type Www struct {
	*sigmaclnt.SigmaClnt
	p        *perf.Perf
	record   bool
	onlyWWW  bool
	job      string
	tracer   *tracing.Tracer
	userc    *rpcclnt.RPCClnt
	searchc  *rpcclnt.RPCClnt
	reservec *rpcclnt.RPCClnt
	profc    *rpcclnt.RPCClnt
	recc     *rpcclnt.RPCClnt
	geodc    *shardedsvcrpcclnt.ShardedSvcRPCClnt
}

// Run starts the server
func RunWww(job string, onlyWWW, runningInDocker bool) error {
	db.DPrintf(db.ALWAYS, "onlyWWW %v runningInDocker %v", onlyWWW, runningInDocker)
	www := &Www{}
	www.record = true
	www.job = job
	sc, err := sigmaclnt.NewSigmaClnt(proc.GetProcEnv())
	if err != nil {
		return err
	}
	www.SigmaClnt = sc
	www.onlyWWW = onlyWWW

	fsl, err := NewFsLib("hotel-wwwd", www.GetDialProxyClnt())
	if err != nil {
		return err
	}
	if !onlyWWW {
		rpcc, err := sprpcclnt.NewRPCClnt(fsl, HOTELUSER)
		if err != nil {
			return err
		}
		www.userc = rpcc
		rpcc, err = sprpcclnt.NewRPCClnt(fsl, HOTELSEARCH)
		if err != nil {
			return err
		}
		www.searchc = rpcc
		rpcc, err = sprpcclnt.NewRPCClnt(fsl, HOTELPROF)
		if err != nil {
			return err
		}
		www.profc = rpcc
		rpcc, err = sprpcclnt.NewRPCClnt(fsl, HOTELRESERVE)
		if err != nil {
			return err
		}
		www.reservec = rpcc
		rpcc, err = sprpcclnt.NewRPCClnt(fsl, HOTELREC)
		if err != nil {
			return err
		}
		www.recc = rpcc
		www.geodc = shardedsvcrpcclnt.NewShardedSvcRPCClnt(fsl, HOTELGEODIR, db.HOTEL_WWW, db.HOTEL_WWW_ERR)
	}

	//	www.tracer = tracing.Init("wwwd", proc.GetSigmaJaegerIP())
	var mux *http.ServeMux
	//	var tmux *tracing.TracedHTTPMux
	//	if TRACING {
	//		tmux = tracing.NewHTTPMux()
	//		tmux.HandleFunc("/user", www.userHandler)
	//		tmux.HandleFunc("/hotels", www.searchHandler)
	//		tmux.HandleFunc("/recommendations", www.recommendHandler)
	//		tmux.HandleFunc("/reservation", www.reservationHandler)
	//		tmux.HandleFunc("/geo", www.geoHandler)
	//	} else {
	mux = http.NewServeMux()
	mux.HandleFunc("/user", www.userHandler)
	mux.HandleFunc("/hotels", www.searchHandler)
	mux.HandleFunc("/recommendations", www.recommendHandler)
	mux.HandleFunc("/reservation", www.reservationHandler)
	mux.HandleFunc("/geo", www.geoHandler)
	mux.HandleFunc("/spin", www.spinHandler)
	mux.HandleFunc("/startrecording", www.startRecordingHandler)
	mux.HandleFunc("/stoprecording", www.stopRecordingHandler)
	//	}

	ep, l, err := www.GetDialProxyClnt().Listen(sp.EXTERNAL_EP, sp.NewTaddr(sp.NO_IP, sp.NO_PORT))
	if err != nil {
		db.DFatalf("Error Listen: %v", err)
	}
	//		if TRACING {
	//			go tmux.Serve(l)
	//		} else {
	go http.Serve(l, mux)
	//		}

	db.DPrintf(db.ALWAYS, "Hotel advertise %v", ep)
	if err = www.MkEndpointFile(JobHTTPAddrsPath(job), ep); err != nil {
		db.DFatalf("MkEndpointFile %v", err)
	}

	perf, err := perf.NewPerf(sc.ProcEnv(), perf.HOTEL_WWW)
	if err != nil {
		db.DFatalf("NewPerf err %v\n", err)
	}
	www.p = perf

	if runningInDocker {
		// If running in Docker (not as a true SigmaOS proc), spin indefinitely
		for {
			time.Sleep(1000 * time.Second)
		}
	}

	if err := www.Started(); err != nil {
		return err
	}

	return www.done()
}

func (s *Www) done() error {
	if err := s.WaitEvict(s.ProcEnv().GetPID()); err != nil {
		return err
	}
	if !s.onlyWWW {
		db.DPrintf(db.HOTEL_WWW_STATS, "\nUserc %v", s.userc.StatsClnt())
		db.DPrintf(db.HOTEL_WWW_STATS, "\nSearchc %v", s.searchc.StatsClnt())
		db.DPrintf(db.HOTEL_WWW_STATS, "\nReservec %v", s.reservec.StatsClnt())
		db.DPrintf(db.HOTEL_WWW_STATS, "\nProfc %v", s.profc.StatsClnt())
		db.DPrintf(db.HOTEL_WWW_STATS, "\nRecc %v", s.recc.StatsClnt())
		db.DPrintf(db.HOTEL_WWW, "Www %v evicted", s.ProcEnv().GetPID())
	}
	//	s.tracer.Flush()
	s.p.Done()
	s.ClntExit(proc.NewStatus(proc.StatusEvicted))
	return nil
}

func (s *Www) userHandler(w http.ResponseWriter, r *http.Request) {
	if s.record {
		defer s.p.TptTick(1.0)
	}
	//	var span trace.Span
	//	var sctx *tproto.SpanContextConfig
	//	if TRACING {
	//		_, span = s.tracer.StartContextSpan(r.Context(), "User")
	//		defer span.End()
	//		sctx = tracing.SpanToContext(span)
	//	}

	w.Header().Set("Access-Control-Allow-Origin", "*")

	username, password := r.URL.Query().Get("username"), r.URL.Query().Get("password")
	//	username := r.FormValue("username")
	//	password := r.FormValue("password")

	if username == "" || password == "" {
		http.Error(w, "Please specify username and password", http.StatusBadRequest)
		return
	}

	var res proto.UserRep

	// Check username and password
	err := s.userc.RPC("Users.CheckUser", &proto.UserReq{
		Name:              username,
		Password:          password,
		SpanContextConfig: nil, //sctx,
	}, &res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	str := "Login successfully!"
	if res.OK == "False" {
		str = "Failed. Please check your username and password. "
	}

	reply := map[string]interface{}{
		"message": str,
	}

	json.NewEncoder(w).Encode(reply)
}

func (s *Www) searchHandler(w http.ResponseWriter, r *http.Request) {
	if s.record {
		defer s.p.TptTick(1.0)
	}
	//	var sctx context.Context
	//	var span trace.Span
	//	if TRACING {
	//		sctx, span = s.tracer.StartContextSpan(r.Context(), "Search")
	//		defer span.End()
	//	}

	w.Header().Set("Access-Control-Allow-Origin", "*")

	//	headerContentTtype := r.Header.Get("Content-Type")
	//	if headerContentTtype != "application/x-www-form-urlencoded" {
	//		db.DPrintf(db.ALWAYS, "format %v", headerContentTtype)
	//		http.Error(w, "not urlencoded", http.StatusUnsupportedMediaType)
	//		return
	//	}

	inDate, outDate := r.URL.Query().Get("inDate"), r.URL.Query().Get("outDate")
	//	inDate := r.FormValue("inDate")
	//	outDate := r.FormValue("outDate")
	if inDate == "" || outDate == "" {
		http.Error(w, "Please specify inDate/outDate params", http.StatusBadRequest)
		return
	}

	// lan/lon from query params
	sLat, sLon := r.URL.Query().Get("lat"), r.URL.Query().Get("lon")
	//	sLat := r.FormValue("lat")
	//	sLon := r.FormValue("lon")
	if sLat == "" || sLon == "" {
		http.Error(w, "Please specify location params", http.StatusBadRequest)
		return
	}

	Lat, _ := strconv.ParseFloat(sLat, 64)
	lat := float32(Lat)
	Lon, _ := strconv.ParseFloat(sLon, 64)
	lon := float32(Lon)

	//	var span2 trace.Span
	//	var sctx2 *tproto.SpanContextConfig
	//	if TRACING {
	//		_, span2 = s.tracer.StartContextSpan(sctx, "Search.Nearby")
	//		sctx2 = tracing.SpanToContext(span2)
	//	}
	var searchRes proto.SearchRep
	searchReq := &proto.SearchReq{
		Lat:               lat,
		Lon:               lon,
		InDate:            inDate,
		OutDate:           outDate,
		SpanContextConfig: nil, //sctx2,
	}
	// search for best hotels
	err := s.searchc.RPC("Search.Nearby", searchReq, &searchRes)
	//	if TRACING {
	//		span2.End()
	//	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	db.DPrintf(db.HOTEL_WWW, "Searchres %v %v\n", searchReq, searchRes)
	// grab locale from query params or default to en
	locale := r.URL.Query().Get("locale")
	//	locale := r.FormValue("locale")
	if locale == "" {
		locale = "en"
	}

	var reserveRes proto.ReserveRep
	//	var span3 trace.Span
	//	var sctx3 *tproto.SpanContextConfig
	//	if TRACING {
	//		_, span3 = s.tracer.StartContextSpan(sctx, "Reserve.CheckAvailability")
	//		sctx3 = tracing.SpanToContext(span3)
	//	}
	err = s.reservec.RPC("Reserve.CheckAvailability", &proto.ReserveReq{
		CustomerName:      "",
		HotelId:           searchRes.HotelIds,
		InDate:            inDate,
		OutDate:           outDate,
		Number:            1,
		SpanContextConfig: nil, //sctx3,
	}, &reserveRes)
	//	if TRACING {
	//		span3.End()
	//	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// hotel profiles
	var profRes proto.ProfRep
	//	var span4 trace.Span
	//	var sctx4 *tproto.SpanContextConfig
	//	if TRACING {
	//		_, span4 = s.tracer.StartContextSpan(sctx, "ProfSrv.GetProfiles")
	//		sctx4 = tracing.SpanToContext(span4)
	//	}
	err = s.profc.RPC("ProfSrv.GetProfiles", &proto.ProfReq{
		HotelIds:          reserveRes.HotelIds,
		Locale:            locale,
		SpanContextConfig: nil, //sctx4,
	}, &profRes)
	//	if TRACING {
	//		span4.End()
	//	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(geoJSONResponse(profRes.Hotels))
}

func (s *Www) recommendHandler(w http.ResponseWriter, r *http.Request) {
	if s.record {
		defer s.p.TptTick(1.0)
	}
	//	var span trace.Span
	//	var sctx *tproto.SpanContextConfig
	//	if TRACING {
	//		_, span = s.tracer.StartContextSpan(r.Context(), "Recommend")
	//		defer span.End()
	//		sctx = tracing.SpanToContext(span)
	//	}

	w.Header().Set("Access-Control-Allow-Origin", "*")

	// lan/lon from query params
	sLat, sLon := r.URL.Query().Get("lat"), r.URL.Query().Get("lon")
	//	sLat := r.FormValue("lat")
	//	sLon := r.FormValue("lon")
	if sLat == "" || sLon == "" {
		http.Error(w, "Please specify location params", http.StatusBadRequest)
		return
	}

	Lat, _ := strconv.ParseFloat(sLat, 64)
	lat := float64(Lat)
	Lon, _ := strconv.ParseFloat(sLon, 64)
	lon := float64(Lon)

	require := r.URL.Query().Get("require")
	//	require := r.FormValue("require")
	if require != "dis" && require != "rate" && require != "price" {
		http.Error(w, "Please specify require params", http.StatusBadRequest)
		return
	}

	// recommend hotels
	var recResp proto.RecRep
	err := s.recc.RPC("Rec.GetRecs", &proto.RecReq{
		Require:           require,
		Lat:               lat,
		Lon:               lon,
		SpanContextConfig: nil, //sctx,
	}, &recResp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// grab locale from query params or default to en
	locale := r.URL.Query().Get("locale")
	//	locale := r.FormValue("locale")
	if locale == "" {
		locale = "en"
	}

	// hotel profiles
	var profResp proto.ProfRep
	err = s.profc.RPC("ProfSrv.GetProfiles", &proto.ProfReq{
		HotelIds:          recResp.HotelIds,
		Locale:            locale,
		SpanContextConfig: nil, //sctx,
	}, &profResp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(geoJSONResponse(profResp.Hotels))
}

func (s *Www) reservationHandler(w http.ResponseWriter, r *http.Request) {
	if s.record {
		defer s.p.TptTick(1.0)
	}
	//	var span trace.Span
	//	var sctx *tproto.SpanContextConfig
	//	if TRACING {
	//		_, span = s.tracer.StartContextSpan(r.Context(), "Reservation")
	//		defer span.End()
	//		sctx = tracing.SpanToContext(span)
	//	}

	w.Header().Set("Access-Control-Allow-Origin", "*")

	inDate, outDate := r.URL.Query().Get("inDate"), r.URL.Query().Get("outDate")
	//	inDate := r.FormValue("inDate")
	//	outDate := r.FormValue("outDate")
	if inDate == "" || outDate == "" {
		http.Error(w, "Please specify inDate/outDate params", http.StatusBadRequest)
		return
	}

	if !checkDataFormat(inDate) || !checkDataFormat(outDate) {
		http.Error(w, "Please check inDate/outDate format (YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	hotelId := r.URL.Query().Get("hotelId")
	//	hotelId := r.FormValue("hotelId")
	if hotelId == "" {
		http.Error(w, "Please specify hotelId params", http.StatusBadRequest)
		return
	}

	customerName := r.URL.Query().Get("customerName")
	//	customerName := r.FormValue("customername")
	if customerName == "" {
		http.Error(w, "Please specify customerName params", http.StatusBadRequest)
		return
	}

	username, password := r.URL.Query().Get("username"), r.URL.Query().Get("password")
	//	username := r.FormValue("username")
	//	password := r.FormValue("password")
	if username == "" || password == "" {
		http.Error(w, "Please specify username and password", http.StatusBadRequest)
		return
	}

	numberOfRoom := 0
	num := r.URL.Query().Get("number")
	//	num := r.FormValue("number")
	if num != "" {
		numberOfRoom, _ = strconv.Atoi(num)
	}

	var res proto.UserRep

	// Check username and password
	err := s.userc.RPC("Users.CheckUser", &proto.UserReq{
		Name:              username,
		Password:          password,
		SpanContextConfig: nil, //sctx,
	}, &res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	str := "Reserve successfully!"
	if res.OK == "False" {
		str = "Failed. Please check your username and password. "
		http.Error(w, str, http.StatusBadRequest)
	}

	// Make reservation
	var resResp proto.ReserveRep
	err = s.reservec.RPC("Reserve.NewReservation", &proto.ReserveReq{
		CustomerName:      customerName,
		HotelId:           []string{hotelId},
		InDate:            inDate,
		OutDate:           outDate,
		Number:            int32(numberOfRoom),
		SpanContextConfig: nil, //sctx,
	}, &resResp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(resResp.HotelIds) == 0 {
		str = "Failed. Already reserved. "
	}

	repl := map[string]interface{}{
		"message": str,
	}

	json.NewEncoder(w).Encode(repl)
}

func (s *Www) geoHandler(w http.ResponseWriter, r *http.Request) {
	if s.record {
		defer s.p.TptTick(1.0)
	}
	//	var span trace.Span
	//	var sctx *tproto.SpanContextConfig
	//	if TRACING {
	//		_, span = s.tracer.StartContextSpan(r.Context(), "Geo")
	//		defer span.End()
	//		sctx = tracing.SpanToContext(span)
	//	}

	w.Header().Set("Access-Control-Allow-Origin", "*")

	// lan/lon from query params
	sLat, sLon := r.URL.Query().Get("lat"), r.URL.Query().Get("lon")
	//	sLat := r.FormValue("lat")
	//	sLon := r.FormValue("lon")
	if sLat == "" || sLon == "" {
		http.Error(w, "Please specify location params", http.StatusBadRequest)
		return
	}

	Lat, _ := strconv.ParseFloat(sLat, 64)
	lat := float32(Lat)
	Lon, _ := strconv.ParseFloat(sLon, 64)
	lon := float32(Lon)

	var gres proto.GeoRep
	greq := proto.GeoReq{
		Lat:               lat,
		Lon:               lon,
		SpanContextConfig: nil, //sctx,
	}
	geoID, err := s.geodc.WaitTimedRandomEntry()
	if err != nil {
		db.DFatalf("choose srv error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rpcc, err := s.geodc.GetClnt(geoID)
	if err != nil {
		db.DFatalf("geo getClnt error: %v", err)
	}
	err = rpcc.RPC("Geo.Nearby", &greq, &gres)
	//	err := s.geoc.RPC("Geo.Nearby", greq, &gres)
	if err != nil {
		db.DFatalf("nearby error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	db.DPrintf(db.HOTEL_WWW, "Geo Nearby: %v %v\n", greq, gres)

	str := "Geo!"

	reply := map[string]interface{}{
		"message": str,
	}

	json.NewEncoder(w).Encode(reply)
}

func (s *Www) spinHandler(w http.ResponseWriter, r *http.Request) {
	if s.record {
		defer s.p.TptTick(1.0)
	}
	//	var span trace.Span
	//	var sctx *tproto.SpanContextConfig
	//	if TRACING {
	//		_, span = s.tracer.StartContextSpan(r.Context(), "Geo")
	//		defer span.End()
	//		sctx = tracing.SpanToContext(span)
	//	}

	w.Header().Set("Access-Control-Allow-Origin", "*")

	// lan/lon from query params
	sNIter := r.URL.Query().Get("n")
	//	sLat := r.FormValue("lat")
	//	sLon := r.FormValue("lon")
	if sNIter == "" {
		http.Error(w, "Please specify number of spin iterations", http.StatusBadRequest)
		return
	}

	nIter, err := strconv.ParseUint(sNIter, 10, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("Err parsing num iterations: %v", err), http.StatusBadRequest)
		return
	}

	db.DPrintf(db.HOTEL_WWW, "spin n %v", nIter)
	start := time.Now()
	x := uint64(0)
	for i := uint64(1); i < nIter; i++ {
		x += i*x + 1
	}
	dur := time.Since(start)
	db.DPrintf(db.HOTEL_WWW, "spin done n %v in %v", nIter, dur)

	str := fmt.Sprintf("Done in %v res %v", dur, x)

	reply := map[string]interface{}{
		"message": str,
	}

	json.NewEncoder(w).Encode(reply)
}

func (s *Www) startRecordingHandler(w http.ResponseWriter, r *http.Request) {
	//	var span trace.Span
	//	var sctx *tproto.SpanContextConfig
	//	if TRACING {
	//		_, span = s.tracer.StartContextSpan(r.Context(), "Geo")
	//		defer span.End()
	//		sctx = tracing.SpanToContext(span)
	//	}

	s.record = true

	w.Header().Set("Access-Control-Allow-Origin", "*")

	db.DPrintf(db.HOTEL_WWW, "Start recording")

	str := "Started recording!"

	reply := map[string]interface{}{
		"message": str,
	}

	json.NewEncoder(w).Encode(reply)
}

func (s *Www) stopRecordingHandler(w http.ResponseWriter, r *http.Request) {
	s.record = false

	w.Header().Set("Access-Control-Allow-Origin", "*")

	db.DPrintf(db.HOTEL_WWW, "Stop recording")

	str := "Stopped recording!"

	reply := map[string]interface{}{
		"message": str,
	}

	// Save results
	s.p.Done()

	json.NewEncoder(w).Encode(reply)
}

// return a geoJSON response that allows google map to plot points directly on map
// https://developers.google.com/maps/documentation/javascript/datalayer#sample_geojson
func geoJSONResponse(hs []*proto.ProfileFlat) map[string]interface{} {
	fs := []interface{}{}

	for _, h := range hs {
		fs = append(fs, map[string]interface{}{
			"type": "Feature",
			"id":   h.HotelId,
			"properties": map[string]string{
				"name":         h.Name,
				"phone_number": h.PhoneNumber,
			},
			"geometry": map[string]interface{}{
				"type": "Point",
				"coordinates": []float32{
					h.Lon,
					h.Lat,
				},
			},
		})
	}

	return map[string]interface{}{
		"type":     "FeatureCollection",
		"features": fs,
	}
}

func checkDataFormat(date string) bool {
	if len(date) != 10 {
		return false
	}
	for i := 0; i < 10; i++ {
		if i == 4 || i == 7 {
			if date[i] != '-' {
				return false
			}
		} else {
			if date[i] < '0' || date[i] > '9' {
				return false
			}
		}
	}
	return true
}

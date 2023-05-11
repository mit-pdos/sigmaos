package hotel

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"

	//	"context"
	//	"go.opentelemetry.io/otel/trace"
	//	tproto "sigmaos/tracing/proto"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/hotel/proto"
	"sigmaos/perf"
	"sigmaos/portclnt"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/tracing"
)

type Www struct {
	*sigmaclnt.SigmaClnt
	p        *perf.Perf
	record   bool
	job      string
	tracer   *tracing.Tracer
	userc    *protdevclnt.ProtDevClnt
	searchc  *protdevclnt.ProtDevClnt
	reservec *protdevclnt.ProtDevClnt
	profc    *protdevclnt.ProtDevClnt
	recc     *protdevclnt.ProtDevClnt
	geoc     *protdevclnt.ProtDevClnt
	pc       *portclnt.PortClnt
}

// Run starts the server
func RunWww(job string, public bool) error {
	www := &Www{}
	www.job = job
	sc, err := sigmaclnt.MkSigmaClnt("hotel-wwwd-" + job)
	if err != nil {
		return err
	}
	www.SigmaClnt = sc
	fsls := MakeFsLibs("hotel-wwwd")
	pdc, err := protdevclnt.MkProtDevClnt(fsls, sp.HOTELUSER)
	if err != nil {
		return err
	}
	www.userc = pdc
	pdc, err = protdevclnt.MkProtDevClnt(fsls, sp.HOTELSEARCH)
	if err != nil {
		return err
	}
	www.searchc = pdc
	pdc, err = protdevclnt.MkProtDevClnt(fsls, sp.HOTELPROF)
	if err != nil {
		return err
	}
	www.profc = pdc
	pdc, err = protdevclnt.MkProtDevClnt(fsls, sp.HOTELRESERVE)
	if err != nil {
		return err
	}
	www.reservec = pdc
	pdc, err = protdevclnt.MkProtDevClnt(fsls, sp.HOTELREC)
	if err != nil {
		return err
	}
	www.recc = pdc
	pdc, err = protdevclnt.MkProtDevClnt(fsls, sp.HOTELGEO)
	if err != nil {
		return err
	}
	www.geoc = pdc

	www.tracer = tracing.Init("wwwd", proc.GetSigmaJaegerIP())
	var mux *http.ServeMux
	//	var tmux *tracing.TracedHTTPMux
	//	if TRACING {
	//		tmux = tracing.MakeHTTPMux()
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
	mux.HandleFunc("/startrecording", www.startRecordingHandler)
	//	}

	if public {
		pc, pi, err := portclnt.MkPortClntPort(www.FsLib)
		if err != nil {
			db.DFatalf("AllocPort err %v", err)
		}
		www.pc = pc
		l, err := net.Listen("tcp", ":"+pi.Pb.RealmPort.String())
		if err != nil {
			db.DFatalf("Error %v Listen: %v", public, err)
		}
		//		if TRACING {
		//			go tmux.Serve(l)
		//		} else {
		go http.Serve(l, mux)
		//		}
		a, err := container.QualifyAddr(l.Addr().String())
		if err != nil {
			db.DFatalf("QualifyAddr %v err %v", a, err)
		}
		if err = pc.AdvertisePort(JobHTTPAddrsPath(job), pi, proc.GetNet(), a); err != nil {
			db.DFatalf("AdvertisePort %v", err)
		}
	} else {
		l, err := net.Listen("tcp", ":0")
		if err != nil {
			db.DFatalf("Error %v Listen: %v", public, err)
		}
		//		if TRACING {
		//			go tmux.Serve(l)
		//		} else {
		go http.Serve(l, mux)
		//		}

		a, err := container.QualifyAddr(l.Addr().String())
		if err != nil {
			db.DFatalf("QualifyAddr %v err %v", a, err)
		}
		db.DPrintf(db.ALWAYS, "Hotel advertise %v jaegerip %v", a, proc.GetSigmaJaegerIP())
		mnt := sp.MkMountService(sp.MkTaddrs([]string{a}))
		if err = www.MountService(JobHTTPAddrsPath(job), mnt); err != nil {
			db.DFatalf("MountService %v", err)
		}
	}

	perf, err := perf.MakePerf(perf.HOTEL_WWW)
	if err != nil {
		db.DFatalf("MakePerf err %v\n", err)
	}
	www.p = perf

	if err := www.Started(); err != nil {
		return err
	}

	return www.done()
}

func (s *Www) done() error {
	if err := s.WaitEvict(proc.GetPid()); err != nil {
		return err
	}
	db.DPrintf(db.HOTEL_WWW_STATS, "\nUserc %v", s.userc.StatsClnt())
	db.DPrintf(db.HOTEL_WWW_STATS, "\nSearchc %v", s.searchc.StatsClnt())
	db.DPrintf(db.HOTEL_WWW_STATS, "\nReservec %v", s.reservec.StatsClnt())
	db.DPrintf(db.HOTEL_WWW_STATS, "\nProfc %v", s.profc.StatsClnt())
	db.DPrintf(db.HOTEL_WWW_STATS, "\nRecc %v", s.recc.StatsClnt())
	db.DPrintf(db.HOTEL_WWW, "Www %v evicted", proc.GetPid())
	s.tracer.Flush()
	s.p.Done()
	s.Exited(proc.MakeStatus(proc.StatusEvicted))
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

	var res proto.UserResult

	// Check username and password
	err := s.userc.RPC("User.CheckUser", &proto.UserRequest{
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
	var searchRes proto.SearchResult
	searchReq := &proto.SearchRequest{
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

	var reserveRes proto.ReserveResult
	//	var span3 trace.Span
	//	var sctx3 *tproto.SpanContextConfig
	//	if TRACING {
	//		_, span3 = s.tracer.StartContextSpan(sctx, "Reserve.CheckAvailability")
	//		sctx3 = tracing.SpanToContext(span3)
	//	}
	err = s.reservec.RPC("Reserve.CheckAvailability", &proto.ReserveRequest{
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
	var profRes proto.ProfResult
	//	var span4 trace.Span
	//	var sctx4 *tproto.SpanContextConfig
	//	if TRACING {
	//		_, span4 = s.tracer.StartContextSpan(sctx, "ProfSrv.GetProfiles")
	//		sctx4 = tracing.SpanToContext(span4)
	//	}
	err = s.profc.RPC("ProfSrv.GetProfiles", &proto.ProfRequest{
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
	var recResp proto.RecResult
	err := s.recc.RPC("Rec.GetRecs", &proto.RecRequest{
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
	var profResp proto.ProfResult
	err = s.profc.RPC("ProfSrv.GetProfiles", &proto.ProfRequest{
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

	var res proto.UserResult

	// Check username and password
	err := s.userc.RPC("User.CheckUser", &proto.UserRequest{
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
	var resResp proto.ReserveResult
	err = s.reservec.RPC("Reserve.MakeReservation", &proto.ReserveRequest{
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

	//XXX
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

	var gres proto.GeoResult
	greq := proto.GeoRequest{
		Lat:               lat,
		Lon:               lon,
		SpanContextConfig: nil, //sctx,
	}
	err := s.geoc.RPC("Geo.Nearby", &greq, &gres)
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

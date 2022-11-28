package hotel

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/fidclnt"
	"sigmaos/fslib"
	"sigmaos/hotel/proto"
	np "sigmaos/ninep"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/protdevclnt"
)

type Www struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	job      string
	userc    *protdevclnt.ProtDevClnt
	searchc  *protdevclnt.ProtDevClnt
	reservec *protdevclnt.ProtDevClnt
	profc    *protdevclnt.ProtDevClnt
	recc     *protdevclnt.ProtDevClnt
	geoc     *protdevclnt.ProtDevClnt
}

// Run starts the server
func RunWww(job string) error {
	www := &Www{}
	www.job = job
	www.FsLib = fslib.MakeFsLib("hotel-wwwd-" + job)
	www.ProcClnt = procclnt.MakeProcClnt(www.FsLib)
	pdc, err := protdevclnt.MkProtDevClnt(www.FsLib, np.HOTELUSER)
	if err != nil {
		return err
	}
	www.userc = pdc
	pdc, err = protdevclnt.MkProtDevClnt(www.FsLib, np.HOTELSEARCH)
	if err != nil {
		return err
	}
	www.searchc = pdc
	pdc, err = protdevclnt.MkProtDevClnt(www.FsLib, np.HOTELPROF)
	if err != nil {
		return err
	}
	www.profc = pdc
	pdc, err = protdevclnt.MkProtDevClnt(www.FsLib, np.HOTELRESERVE)
	if err != nil {
		return err
	}
	www.reservec = pdc

	pdc, err = protdevclnt.MkProtDevClnt(www.FsLib, np.HOTELREC)
	if err != nil {
		return err
	}
	www.recc = pdc

	pdc, err = protdevclnt.MkProtDevClnt(www.FsLib, np.HOTELGEO)
	if err != nil {
		return err
	}
	www.geoc = pdc

	http.HandleFunc("/user", www.userHandler)
	http.HandleFunc("/hotels", www.searchHandler)
	http.HandleFunc("/recommendations", www.recommendHandler)
	http.HandleFunc("/reservation", www.reservationHandler)
	http.HandleFunc("/geo", www.geoHandler)

	ip, err := fidclnt.LocalIP()
	if err != nil {
		db.DFatalf("Error LocalIP: %v", err)
	}

	l, err := net.Listen("tcp", ip+":0")
	if err != nil {
		db.DFatalf("Error Listen: %v", err)
	}

	go func() {
		db.DFatalf("%v", http.Serve(l, nil))
	}()

	// Write a file for clients to discover the server's address.
	p := JobHTTPAddrsPath(job)
	if err := www.PutFileJson(p, 0777, []string{l.Addr().String()}); err != nil {
		db.DFatalf("Error PutFileJson addrs %v", err)
	}

	pf := perf.MakePerf("HOTEL_WWW")
	defer pf.Done()

	if err := www.Started(); err != nil {
		return err
	}

	return www.done()
}

func (s *Www) done() error {
	if err := s.WaitEvict(proc.GetPid()); err != nil {
		return err
	}
	db.DPrintf("HOTEL_WWW_STATS", "\nUserc %v", s.userc.StatsClnt())
	db.DPrintf("HOTEL_WWW_STATS", "\nSearchc %v", s.searchc.StatsClnt())
	db.DPrintf("HOTEL_WWW_STATS", "\nReservec %v", s.reservec.StatsClnt())
	db.DPrintf("HOTEL_WWW_STATS", "\nProfc %v", s.profc.StatsClnt())
	db.DPrintf("HOTEL_WWW_STATS", "\nRecc %v", s.recc.StatsClnt())
	s.Exited(proc.MakeStatus(proc.StatusEvicted))
	return nil
}

func (s *Www) userHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	username, password := r.URL.Query().Get("username"), r.URL.Query().Get("password")
	//	username := r.FormValue("username")
	//	password := r.FormValue("password")

	if username == "" || password == "" {
		http.Error(w, "Please specify username and password", http.StatusBadRequest)
		return
	}

	var res UserResult

	// Check username and password
	err := s.userc.RPC("User.CheckUser", UserRequest{
		Name:     username,
		Password: password,
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

	var searchRes proto.SearchResult
	searchReq := &proto.SearchRequest{
		Lat:     lat,
		Lon:     lon,
		InDate:  inDate,
		OutDate: outDate,
	}
	// search for best hotels
	err := s.searchc.RPCproto("Search.Nearby", searchReq, &searchRes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	db.DPrintf("HOTELWWWD", "Searchres %v %v\n", searchReq, searchRes)
	// grab locale from query params or default to en
	locale := r.URL.Query().Get("locale")
	//	locale := r.FormValue("locale")
	if locale == "" {
		locale = "en"
	}

	var reserveRes proto.ReserveResult
	err = s.reservec.RPC("Reserve.CheckAvailability", &proto.ReserveRequest{
		CustomerName: "",
		HotelId:      searchRes.HotelIds,
		InDate:       inDate,
		OutDate:      outDate,
		Number:       1,
	}, &reserveRes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// hotel profiles
	var profRes proto.ProfResult
	err = s.profc.RPCproto("ProfSrv.GetProfiles", &proto.ProfRequest{
		HotelIds: reserveRes.HotelIds,
		Locale:   locale,
	}, &profRes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(geoJSONResponse(profRes.Hotels))
}

func (s *Www) recommendHandler(w http.ResponseWriter, r *http.Request) {
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
	err := s.recc.RPCproto("Rec.GetRecs", &proto.RecRequest{
		Require: require,
		Lat:     lat,
		Lon:     lon,
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
	err = s.profc.RPCproto("ProfSrv.GetProfiles", &proto.ProfRequest{
		HotelIds: recResp.HotelIds,
		Locale:   locale,
	}, &profResp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(geoJSONResponse(profResp.Hotels))
}

func (s *Www) reservationHandler(w http.ResponseWriter, r *http.Request) {
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

	var res UserResult

	// Check username and password
	err := s.userc.RPC("User.CheckUser", UserRequest{
		Name:     username,
		Password: password,
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
		CustomerName: customerName,
		HotelId:      []string{hotelId},
		InDate:       inDate,
		OutDate:      outDate,
		Number:       int32(numberOfRoom),
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
		Lat: lat,
		Lon: lon,
	}
	err := s.geoc.RPCproto("Geo.Nearby", &greq, &gres)
	//	err := s.geoc.RPC("Geo.Nearby", greq, &gres)
	if err != nil {
		db.DFatalf("nearby error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	db.DPrintf("HOTEL_WWW", "Geo Nearby: %v %v\n", greq, gres)

	str := "Geo!"

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

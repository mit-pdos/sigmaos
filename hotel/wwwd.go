package hotel

import (
	"encoding/json"
	"net/http"
	"strconv"

	"sigmaos/fslib"
	np "sigmaos/ninep"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/protdevclnt"
)

type Www struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	userc    *protdevclnt.ProtDevClnt
	searchc  *protdevclnt.ProtDevClnt
	reservec *protdevclnt.ProtDevClnt
	profc    *protdevclnt.ProtDevClnt
	recc     *protdevclnt.ProtDevClnt
}

// Run starts the server
func RunWww(n string) error {
	www := &Www{}
	www.FsLib = fslib.MakeFsLib(n)
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

	if err := www.Started(); err != nil {
		return err
	}
	srv := &http.Server{
		Addr:    ":8090",
		Handler: nil,
	}
	http.HandleFunc("/user", www.userHandler)
	http.HandleFunc("/hotels", www.searchHandler)
	http.HandleFunc("/recommendations", www.recommendHandler)
	http.HandleFunc("/reservation", www.reservationHandler)
	go func() {
		srv.ListenAndServe()
	}()
	return www.done()
}

func (s *Www) done() error {
	if err := s.WaitEvict(proc.GetPid()); err != nil {
		return err
	}
	s.Exited(proc.MakeStatus(proc.StatusEvicted))
	return nil
}

func (s *Www) userHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	username := r.FormValue("username")
	password := r.FormValue("password")

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

	headerContentTtype := r.Header.Get("Content-Type")
	if headerContentTtype != "application/x-www-form-urlencoded" {
		http.Error(w, "not urlencoded", http.StatusUnsupportedMediaType)
		return
	}

	inDate := r.FormValue("inDate")
	outDate := r.FormValue("outDate")
	if inDate == "" || outDate == "" {
		http.Error(w, "Please specify inDate/outDate params", http.StatusBadRequest)
		return
	}

	// lan/lon from query params
	sLat := r.FormValue("lat")
	sLon := r.FormValue("lon")
	if sLat == "" || sLon == "" {
		http.Error(w, "Please specify location params", http.StatusBadRequest)
		return
	}

	Lat, _ := strconv.ParseFloat(sLat, 64)
	lat := float64(Lat)
	Lon, _ := strconv.ParseFloat(sLon, 64)
	lon := float64(Lon)

	var searchRes SearchResult
	// search for best hotels
	err := s.searchc.RPC("Search.Nearby", SearchRequest{
		Lat:     lat,
		Lon:     lon,
		InDate:  inDate,
		OutDate: outDate,
	}, &searchRes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// grab locale from query params or default to en
	locale := r.FormValue("locale")
	if locale == "" {
		locale = "en"
	}

	var reserveRes ReserveResult
	err = s.reservec.RPC("Reserve.CheckAvailability", &ReserveRequest{
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
	var profRes ProfResult
	err = s.profc.RPC("ProfSrv.GetProfiles", ProfRequest{
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
	sLat := r.FormValue("lat")
	sLon := r.FormValue("lon")
	if sLat == "" || sLon == "" {
		http.Error(w, "Please specify location params", http.StatusBadRequest)
		return
	}

	Lat, _ := strconv.ParseFloat(sLat, 64)
	lat := float64(Lat)
	Lon, _ := strconv.ParseFloat(sLon, 64)
	lon := float64(Lon)

	require := r.FormValue("require")
	if require != "dis" && require != "rate" && require != "price" {
		http.Error(w, "Please specify require params", http.StatusBadRequest)
		return
	}

	// recommend hotels
	var recResp RecResult
	err := s.recc.RPC("Rec.GetRecs", &RecRequest{
		Require: require,
		Lat:     lat,
		Lon:     lon,
	}, &recResp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// grab locale from query params or default to en
	locale := r.FormValue("locale")
	if locale == "" {
		locale = "en"
	}

	// hotel profiles
	var profResp ProfResult
	err = s.profc.RPC("ProfSrv.GetProfiles", &ProfRequest{
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

	inDate := r.FormValue("inDate")
	outDate := r.FormValue("outDate")
	if inDate == "" || outDate == "" {
		http.Error(w, "Please specify inDate/outDate params", http.StatusBadRequest)
		return
	}

	if !checkDataFormat(inDate) || !checkDataFormat(outDate) {
		http.Error(w, "Please check inDate/outDate format (YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	hotelId := r.FormValue("hotelId")
	if hotelId == "" {
		http.Error(w, "Please specify hotelId params", http.StatusBadRequest)
		return
	}

	customerName := r.FormValue("customername")
	if customerName == "" {
		http.Error(w, "Please specify customerName params", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	if username == "" || password == "" {
		http.Error(w, "Please specify username and password", http.StatusBadRequest)
		return
	}

	numberOfRoom := 0
	num := r.FormValue("number")
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
	}

	// Make reservation
	var resResp ReserveResult
	err = s.reservec.RPC("Reserve.MakeReservation", &ReserveRequest{
		CustomerName: customerName,
		HotelId:      []string{hotelId},
		InDate:       inDate,
		OutDate:      outDate,
		Number:       numberOfRoom,
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

// return a geoJSON response that allows google map to plot points directly on map
// https://developers.google.com/maps/documentation/javascript/datalayer#sample_geojson
func geoJSONResponse(hs []*ProfileFlat) map[string]interface{} {
	fs := []interface{}{}

	for _, h := range hs {
		fs = append(fs, map[string]interface{}{
			"type": "Feature",
			"id":   h.Id,
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

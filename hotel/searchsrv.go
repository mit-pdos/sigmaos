package hotel

import (
	"encoding/json"
	"log"

	"sigmaos/fs"
	"sigmaos/fslib"
	np "sigmaos/ninep"
	"sigmaos/protdevclnt"
	"sigmaos/protdevsrv"
)

type SearchRequest struct {
	Lat     float64
	Lon     float64
	InDate  string
	OutDate string
}

type SearchResult struct {
	HotelIds []string
}

type Search struct {
	fslib *fslib.FsLib
	ratec *protdevclnt.ProtDevClnt
	geoc  *protdevclnt.ProtDevClnt
}

// Run starts the server
func RunSearchSrv(n string) error {
	s := &Search{}
	s.fslib = fslib.MakeFsLib(n)
	pdc, err := protdevclnt.MkProtDevClnt(s.fslib, np.HOTELRATE)
	if err != nil {
		return err
	}
	s.ratec = pdc
	pdc, err = protdevclnt.MkProtDevClnt(s.fslib, np.HOTELGEO)
	if err != nil {
		return err
	}
	s.geoc = pdc
	protdevsrv.Run(np.HOTELSEARCH, s.mkStream)
	return nil
}

type StreamSearch struct {
	rep    []byte
	search *Search
}

func (search *Search) mkStream() (fs.File, *np.Err) {
	st := &StreamSearch{}
	st.search = search
	return st, nil
}

// XXX wait on close before processing data?
func (st *StreamSearch) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	var args SearchRequest
	err := json.Unmarshal(b, &args)
	log.Printf("search %v\n", args)
	res, err := st.search.Nearby(&args)
	if err != nil {
		return 0, np.MkErrError(err)
	}
	st.rep, err = json.Marshal(res)
	if err != nil {
		return 0, np.MkErrError(err)
	}
	return np.Tsize(len(b)), nil
}

// XXX incremental read
func (st *StreamSearch) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	if len(st.rep) == 0 || off > 0 {
		return nil, nil
	}
	return st.rep, nil
}

// Nearby returns ids of nearby hotels order by results of ratesrv
func (s *Search) Nearby(req *SearchRequest) (*SearchResult, error) {
	res := new(SearchResult)
	var gres GeoResult
	err := s.geoc.RPCJson(&GeoRequest{
		Lat: req.Lat,
		Lon: req.Lon,
	}, &gres)
	if err != nil {
		log.Fatalf("nearby error: %v", err)
	}

	log.Printf("gres %v\n", gres.HotelIds)

	// find rates for hotels
	var rres RateResult
	err = s.ratec.RPCJson(&RateRequest{
		HotelIds: gres.HotelIds,
		InDate:   req.InDate,
		OutDate:  req.OutDate,
	}, &rres)
	if err != nil {
		log.Fatalf("rates error: %v", err)
	}

	for _, ratePlan := range rres.RatePlans {
		res.HotelIds = append(res.HotelIds, ratePlan.HotelId)
	}

	return res, nil
}

package hotel

import (
	"log"

	db "sigmaos/debug"
	np "sigmaos/ninep"
	"sigmaos/perf"
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
	ratec *protdevclnt.ProtDevClnt
	geoc  *protdevclnt.ProtDevClnt
	pds   *protdevsrv.ProtDevSrv
}

// Run starts the server
func RunSearchSrv(n string) error {
	s := &Search{}
	pds, err := protdevsrv.MakeProtDevSrv(np.HOTELSEARCH, s)
	if err != nil {
		return err
	}
	pdc, err := protdevclnt.MkProtDevClnt(pds.FsLib, np.HOTELRATE)
	if err != nil {
		return err
	}
	s.ratec = pdc
	pdc, err = protdevclnt.MkProtDevClnt(pds.FsLib, np.HOTELGEO)
	if err != nil {
		return err
	}
	s.geoc = pdc

	p := perf.MakePerf("HOTEL_SEARCH")
	defer p.Done()

	return pds.RunServer()
}

// Nearby returns ids of nearby hotels order by results of ratesrv
func (s *Search) Nearby(req SearchRequest, res *SearchResult) error {
	var gres GeoResult
	greq := GeoRequest{
		Lat: req.Lat,
		Lon: req.Lon,
	}
	err := s.geoc.RPC("Geo.Nearby", greq, &gres)
	if err != nil {
		log.Fatalf("nearby error: %v", err)
	}

	db.DPrintf("HOTELSEARCH", "Search Nearby: %v %v\n", greq, gres)

	// find rates for hotels
	var rres RateResult
	rreq := RateRequest{
		HotelIds: gres.HotelIds,
		InDate:   req.InDate,
		OutDate:  req.OutDate,
	}
	err = s.ratec.RPC("Rate.GetRates", rreq, &rres)
	if err != nil {
		log.Fatalf("rates error: %v", err)
	}

	db.DPrintf("HOTELSEARCH", "Search Getrates: %v %v\n", rreq, rres)

	for _, ratePlan := range rres.RatePlans {
		res.HotelIds = append(res.HotelIds, ratePlan.HotelId)
	}

	return nil
}

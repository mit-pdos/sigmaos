package hotel

import (
	"log"

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
	ratec *protdevclnt.ProtDevClnt
	geoc  *protdevclnt.ProtDevClnt
}

// Run starts the server
func RunSearchSrv(n string) error {
	s := &Search{}
	pds := protdevsrv.MakeProtDevSrv(np.HOTELSEARCH, s)
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
	return pds.RunServer()
}

// Nearby returns ids of nearby hotels order by results of ratesrv
func (s *Search) Nearby(req SearchRequest, res *SearchResult) error {
	var gres GeoResult
	err := s.geoc.RPC("Geo.Nearby", GeoRequest{
		Lat: req.Lat,
		Lon: req.Lon,
	}, &gres)
	if err != nil {
		log.Fatalf("nearby error: %v", err)
	}

	log.Printf("gRes %v\n", gres.HotelIds)

	// find rates for hotels
	var rres RateResult
	err = s.ratec.RPC("Rate.GetRates", RateRequest{
		HotelIds: gres.HotelIds,
		InDate:   req.InDate,
		OutDate:  req.OutDate,
	}, &rres)
	if err != nil {
		log.Fatalf("rates error: %v", err)
	}

	log.Printf("rres %v\n", rres.RatePlans)

	for _, ratePlan := range rres.RatePlans {
		res.HotelIds = append(res.HotelIds, ratePlan.HotelId)
	}

	return nil
}

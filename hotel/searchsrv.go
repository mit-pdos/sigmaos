package hotel

import (
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/hotel/proto"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/protdevclnt"
	"sigmaos/protdevsrv"
	sp "sigmaos/sigmap"
	"sigmaos/tracing"
)

type Search struct {
	ratec  *protdevclnt.ProtDevClnt
	geoc   *protdevclnt.ProtDevClnt
	pds    *protdevsrv.ProtDevSrv
	tracer *tracing.Tracer
}

// Run starts the server
func RunSearchSrv(n string, public bool) error {
	s := &Search{}
	pds, err := protdevsrv.MakeProtDevSrvPublic(sp.HOTELSEARCH, s, public)
	if err != nil {
		return err
	}
	pdc, err := protdevclnt.MkProtDevClnt(pds.SigmaClnt().FsLib, sp.HOTELRATE)
	if err != nil {
		return err
	}
	s.ratec = pdc
	pdc, err = protdevclnt.MkProtDevClnt(pds.SigmaClnt().FsLib, sp.HOTELGEO)
	if err != nil {
		return err
	}
	s.geoc = pdc

	p, err := perf.MakePerf(perf.HOTEL_SEARCH)
	if err != nil {
		db.DFatalf("MakePerf err %v\n", err)
	}
	s.tracer = tracing.Init("search", proc.GetSigmaJaegerIP())

	defer s.tracer.Flush()
	defer p.Done()

	return pds.RunServer()
}

// Nearby returns ids of nearby hotels order by results of ratesrv
func (s *Search) Nearby(ctx fs.CtxI, req proto.SearchRequest, res *proto.SearchResult) error {
	sctx, span := s.tracer.StartRPCSpan(&req, "Nearby")
	defer span.End()

	_, span2 := s.tracer.StartContextSpan(sctx, "Geo.Nearby")
	var gres proto.GeoResult
	greq := &proto.GeoRequest{
		Lat:               req.Lat,
		Lon:               req.Lon,
		SpanContextConfig: tracing.SpanToContext(span2),
	}
	err := s.geoc.RPC("Geo.Nearby", greq, &gres)
	span2.End()
	if err != nil {
		db.DFatalf("nearby error: %v", err)
	}

	db.DPrintf(db.HOTEL_SEARCH, "Search Nearby: %v %v\n", greq, gres)

	// find rates for hotels
	_, span3 := s.tracer.StartContextSpan(sctx, "Rate.GetRates")
	var rres proto.RateResult
	rreq := &proto.RateRequest{
		HotelIds:          gres.HotelIds,
		InDate:            req.InDate,
		OutDate:           req.OutDate,
		SpanContextConfig: tracing.SpanToContext(span3),
	}
	err = s.ratec.RPC("Rate.GetRates", rreq, &rres)
	span3.End()
	if err != nil {
		db.DFatalf("rates error: %v", err)
	}

	db.DPrintf(db.HOTEL_SEARCH, "Search Getrates: %v %v\n", rreq, rres)

	for _, ratePlan := range rres.RatePlans {
		res.HotelIds = append(res.HotelIds, ratePlan.HotelId)
	}

	return nil
}

package hotel

import (
	//	"context"

	//	"go.opentelemetry.io/otel/trace"
	//	tproto "sigmaos/tracing/proto"

	"sigmaos/apps/hotel/proto"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/proc"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/rpcdirclnt"
	"sigmaos/sigmasrv"
	"sigmaos/tracing"
	"sigmaos/util/perf"
)

type Search struct {
	ratec  *rpcclnt.RPCClnt
	geodc  *rpcdirclnt.RPCDirClnt
	pds    *sigmasrv.SigmaSrv
	tracer *tracing.Tracer
}

// Run starts the server
func RunSearchSrv(n string) error {
	s := &Search{}
	ssrv, err := sigmasrv.NewSigmaSrv(HOTELSEARCH, s, proc.GetProcEnv())
	if err != nil {
		return err
	}
	fsl, err := NewFsLib(HOTELSEARCH, ssrv.MemFs.SigmaClnt().GetDialProxyClnt())
	if err != nil {
		return err
	}
	rpcc, err := sprpcclnt.NewRPCClnt(fsl, HOTELRATE)
	if err != nil {
		db.DFatalf("Err new rpcclnt rate: %v", err)
		return err
	}
	s.ratec = rpcc

	s.geodc = rpcdirclnt.NewRPCDirClnt(fsl, HOTELGEODIR, db.HOTEL_GEO, db.HOTEL_GEO_ERR)

	p, err := perf.NewPerf(ssrv.MemFs.SigmaClnt().ProcEnv(), perf.HOTEL_SEARCH)
	if err != nil {
		db.DFatalf("NewPerf err %v\n", err)
	}
	//	s.tracer = tracing.Init("search", proc.GetSigmaJaegerIP())
	//	defer s.tracer.Flush()
	defer p.Done()

	return ssrv.RunServer()
}

// Nearby returns ids of nearby hotels order by results of ratesrv
func (s *Search) Nearby(ctx fs.CtxI, req proto.SearchRequest, res *proto.SearchResult) error {
	//	var sctx context.Context
	//	var span trace.Span
	//	if TRACING {
	//		sctx, span = s.tracer.StartRPCSpan(&req, "Nearby")
	//		defer span.End()
	//	}

	//	var span2 trace.Span
	//	var sctx2 *tproto.SpanContextConfig
	//	if TRACING {
	//		_, span2 = s.tracer.StartContextSpan(sctx, "Geo.Nearby")
	//		sctx2 = tracing.SpanToContext(span2)
	//	}
	var gres proto.GeoResult
	greq := &proto.GeoRequest{
		Lat:               req.Lat,
		Lon:               req.Lon,
		SpanContextConfig: nil, //sctx2,
	}
	geoID, err := s.geodc.WaitTimedRandomEntry()
	if err != nil {
		db.DFatalf("choose srv error: %v", err)
	}
	rpcc, err := s.geodc.GetClnt(geoID)
	if err != nil {
		db.DFatalf("geo getClnt error: %v", err)
	}
	err = rpcc.RPC("Geo.Nearby", greq, &gres)
	//	if TRACING {
	//		span2.End()
	//	}
	if err != nil {
		db.DFatalf("nearby error: %v", err)
	}

	db.DPrintf(db.HOTEL_SEARCH, "Search Nearby: %v %v\n", greq, gres)

	// find rates for hotels
	//	var span3 trace.Span
	//	var sctx3 *tproto.SpanContextConfig
	//	if TRACING {
	//		_, span3 = s.tracer.StartContextSpan(sctx, "Rate.GetRates")
	//		sctx3 = tracing.SpanToContext(span3)
	//	}
	var rres proto.RateResult
	rreq := &proto.RateRequest{
		HotelIds:          gres.HotelIds,
		InDate:            req.InDate,
		OutDate:           req.OutDate,
		SpanContextConfig: nil, //sctx3,
	}
	err = s.ratec.RPC("Rate.GetRates", rreq, &rres)
	//	if TRACING {
	//		span3.End()
	//	}
	if err != nil {
		db.DFatalf("rates error: %v", err)
	}

	db.DPrintf(db.HOTEL_SEARCH, "Search Getrates: %v %v\n", rreq, rres)

	for _, ratePlan := range rres.RatePlans {
		res.HotelIds = append(res.HotelIds, ratePlan.HotelId)
	}

	return nil
}

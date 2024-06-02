package hotel

import (
	//	"context"

	//	"go.opentelemetry.io/otel/trace"
	//	tproto "sigmaos/tracing/proto"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/hotel/proto"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/rpcclnt"
	"sigmaos/sigmarpcchan"
	"sigmaos/sigmasrv"
	"sigmaos/tracing"
)

type Search struct {
	ratec  *rpcclnt.RPCClnt
	geoc   *rpcclnt.RPCClnt
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
	fsls, err := NewFsLibs(HOTELSEARCH, ssrv.MemFs.SigmaClnt().GetNetProxyClnt())
	if err != nil {
		return err
	}
	ch, err := sigmarpcchan.NewSigmaRPCCh(fsls, HOTELRATE)
	if err != nil {
		db.DFatalf("Err new rpcclnt rate: %v", err)
		return err
	}
	rpcc := rpcclnt.NewRPCClnt(ch)
	s.ratec = rpcc
	ch, err = sigmarpcchan.NewSigmaRPCCh(fsls, HOTELGEO)
	if err != nil {
		db.DFatalf("Err new rpcclnt geo: %v", err)
		return err
	}
	rpcc = rpcclnt.NewRPCClnt(ch)
	s.geoc = rpcc

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
	err := s.geoc.RPC("Geo.Nearby", greq, &gres)
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

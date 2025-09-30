package hotel

import (
	"path/filepath"
	//	"context"

	//	"go.opentelemetry.io/otel/trace"
	//	tproto "sigmaos/util/tracing/proto"

	"sigmaos/api/fs"
	"sigmaos/apps/epcache"
	epcacheclnt "sigmaos/apps/epcache/clnt"
	"sigmaos/apps/hotel/proto"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/rpc"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/util/perf"
	"sigmaos/util/tracing"
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

	epcc, err := epcacheclnt.NewEndpointCacheClnt(fsl)
	if err != nil {
		return err
	}
	eps, _, err := epcc.GetEndpoints(HOTELGEODIR, epcache.NO_VERSION)
	if err != nil {
		return err
	}
	pn := "name/geosrv"
	if err := fsl.MountTree(sp.NewEndpointFromProto(eps[0].EndpointProto), rpc.RPC, filepath.Join(pn, rpc.RPC)); err != nil {
		return err
	}
	rpcc, err = sprpcclnt.NewRPCClnt(fsl, pn)
	if err != nil {
		return err
	}
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
func (s *Search) Nearby(ctx fs.CtxI, req proto.SearchReq, res *proto.SearchRep) error {
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
	var gres proto.GeoRep
	greq := &proto.GeoReq{
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
	var rres proto.RateRep
	rreq := &proto.RateReq{
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

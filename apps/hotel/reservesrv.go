package hotel

import (
	"context"
	"fmt"
	"strconv"
	"time"

	//	"go.opentelemetry.io/otel/trace"

	"sigmaos/apps/hotel/proto"
	"sigmaos/apps/cache"
	cacheproto "sigmaos/apps/cache/proto"
	dbclnt "sigmaos/db/clnt"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/perf"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/tracing"
)

const ()

type Reservation struct {
	HotelID  string
	Customer string
	InDate   string
	OutDate  string
	Number   int
}

type Number struct {
	HotelId string
	Number  int
}

type Reserve struct {
	dbc    *dbclnt.DbClnt
	cachec cache.CacheClnt
	tracer *tracing.Tracer
}

func (s *Reserve) initDb() error {
	q := fmt.Sprintf("truncate number;")
	err := s.dbc.Exec(q)
	if err != nil {
		return err
	}
	q = fmt.Sprintf("truncate reservation;")
	err = s.dbc.Exec(q)
	if err != nil {
		return err
	}

	q = fmt.Sprintf("INSERT INTO reservation (hotelid, customer, indate, outdate, number) VALUES ('%s', '%s', '%s', '%s', '%d');", "4", "Alice", "2015-04-09", "2015-04-10", 1)
	err = s.dbc.Exec(q)
	if err != nil {
		return err
	}

	for i := 1; i < 7; i++ {
		q = fmt.Sprintf("INSERT INTO number (hotelid, number) VALUES ('%v', '%v');",
			strconv.Itoa(i), 200)
		err = s.dbc.Exec(q)
		if err != nil {
			return err
		}
	}
	for i := 7; i <= nhotel; i++ {
		hotel_id := strconv.Itoa(i)
		room_num := 200
		if i%3 == 1 {
			room_num = 300
		} else if i%3 == 2 {
			room_num = 250
		}
		q = fmt.Sprintf("INSERT INTO number (hotelid, number) VALUES ('%v', '%v');",
			hotel_id, room_num)
		err = s.dbc.Exec(q)
		if err != nil {
			return err
		}
	}
	return nil
}

func RunReserveSrv(job string, cache string) error {
	r := &Reserve{}
	ssrv, err := sigmasrv.NewSigmaSrv(HOTELRESERVE, r, proc.GetProcEnv())
	if err != nil {
		return err
	}
	dbc, err := dbclnt.NewDbClnt(ssrv.MemFs.SigmaClnt().FsLib, sp.DBD)
	if err != nil {
		return err
	}
	r.dbc = dbc
	fsls, err := NewFsLibs(HOTELRESERVE, ssrv.MemFs.SigmaClnt().GetNetProxyClnt())
	if err != nil {
		return err
	}
	cachec, err := NewCacheClnt(cache, fsls, job)
	if err != nil {
		return err
	}
	r.cachec = cachec
	err = r.initDb()
	if err != nil {
		return err
	}
	p, err := perf.NewPerf(ssrv.MemFs.SigmaClnt().ProcEnv(), perf.HOTEL_RESERVE)
	if err != nil {
		db.DFatalf("NewPerf err %v\n", err)
	}
	defer p.Done()
	//	if TRACING {
	//		r.tracer = tracing.Init("reserve", proc.GetSigmaJaegerIP())
	//		defer r.tracer.Flush()
	//	}
	return ssrv.RunServer()
}

// checkAvailability checks if given information is available
func (s *Reserve) checkAvailability(sctx context.Context, hotelId string, req proto.ReserveRequest) (bool, map[string]int, error) {
	inDate, _ := time.Parse(
		time.RFC3339,
		req.InDate+"T12:00:00+00:00")

	outDate, _ := time.Parse(
		time.RFC3339,
		req.OutDate+"T12:00:00+00:00")

	num_date := make(map[string]int)
	indate := inDate.String()[0:10]
	for inDate.Before(outDate) {
		// check reservations
		count := 0
		inDate = inDate.AddDate(0, 0, 1)
		outdate := inDate.String()[0:10]

		key := hotelId + "_" + indate + "_" + outdate

		var reserves []Reservation
		//		var span trace.Span
		//		if TRACING {
		//			_, span = s.tracer.StartContextSpan(sctx, "Cache.Get")
		//		}
		cnt := &cacheproto.CacheInt{}
		err := s.cachec.Get(key, cnt)
		//		if TRACING {
		//			//			err = s.cachec.GetTraced(tracing.SpanToContext(span), key, cnt)
		//			span.End()
		//		}
		count = int(cnt.Val)
		if err != nil {
			if !cache.IsMiss(err) {
				return false, nil, err
			}
			db.DPrintf(db.HOTEL_RESERVE, "Check: cache miss res: key %v\n", key)
			q := fmt.Sprintf("SELECT * from reservation where hotelid='%s' AND indate='%s' AND outdate='%s';", hotelId, indate, outdate)
			//			var dbspan trace.Span
			//			if TRACING {
			//				_, dbspan = s.tracer.StartContextSpan(sctx, "db.Query")
			//			}
			err := s.dbc.Query(q, &reserves)
			//			if TRACING {
			//				dbspan.End()
			//			}
			if err != nil {
				return false, nil, err
			}
			for _, r := range reserves {
				count += r.Number
			}
			//			var span trace.Span
			//			if TRACING {
			//				_, span = s.tracer.StartContextSpan(sctx, "Cache.Put")
			//			}
			err = s.cachec.Put(key, &cacheproto.CacheInt{Val: int64(count)})
			//			if TRACING {
			//				//				err = s.cachec.PutTraced(tracing.SpanToContext(span), key, &cacheproto.CacheInt{Val: int64(count)})
			//				span.End()
			//			}
			if err != nil {
				return false, nil, err
			}
		}

		num_date[key] = count + int(req.Number)

		// check capacity
		hotel_cap := 0
		key = hotelId + "_cap"
		//		var span2 trace.Span
		//		if TRACING {
		//			_, span2 = s.tracer.StartContextSpan(sctx, "Cache.Get")
		//		}
		hc := &cacheproto.CacheInt{}
		err = s.cachec.Get(key, hc)
		hotel_cap = int(hc.Val)
		//		if TRACING {
		//			//		err = s.cachec.GetTraced(tracing.SpanToContext(span2), key, hc)
		//			span2.End()
		//		}
		if err != nil {
			if !cache.IsMiss(err) {
				return false, nil, err
			}
			db.DPrintf(db.HOTEL_RESERVE, "Check: cache miss id: key %v\n", key)
			var nums []Number
			q := fmt.Sprintf("SELECT * from number where hotelid='%s';", hotelId)
			//			var dbspan trace.Span
			//			if TRACING {
			//				_, dbspan = s.tracer.StartContextSpan(sctx, "db.Query")
			//			}
			err = s.dbc.Query(q, &nums)
			//			if TRACING {
			//				dbspan.End()
			//			}
			if err != nil {
				return false, nil, err
			}
			if len(nums) == 0 {
				return false, nil, fmt.Errorf("Unknown %v", hotelId)
			}
			hotel_cap = nums[0].Number
			//			var span trace.Span
			//			if TRACING {
			//				_, span = s.tracer.StartContextSpan(sctx, "Cache.PUt")
			//			}
			err = s.cachec.Put(key, &cacheproto.CacheInt{Val: int64(hotel_cap)})
			//			if TRACING {
			//				//			err = s.cachec.PutTraced(tracing.SpanToContext(span), key, &cacheproto.CacheInt{Val: int64(hotel_cap)})
			//				span.End()
			//			}
			if err != nil {
				return false, nil, err
			}
		}
		if count+int(req.Number) > hotel_cap {
			return false, nil, nil
		}
		indate = outdate
	}
	return true, num_date, nil
}

// NewReservation news a reservation based on given information
// XXX make check and reservation atomic
func (s *Reserve) NewReservation(ctx fs.CtxI, req proto.ReserveRequest, res *proto.ReserveResult) error {
	var sctx context.Context
	//	var span trace.Span
	//	if TRACING {
	//		sctx, span = s.tracer.StartRPCSpan(&req, "NewReservation")
	//		defer span.End()
	//	} else {
	sctx = context.TODO()
	//	}

	hotelId := req.HotelId[0]
	res.HotelIds = make([]string, 0)
	b, date_num, err := s.checkAvailability(sctx, hotelId, req)
	if err != nil {
		return err
	}
	if !b {
		return nil
	}

	// update reservation number
	db.DPrintf(db.HOTEL_RESERVE, "Update cache %v\n", date_num)
	for key, cnt := range date_num {
		//		var span2 trace.Span
		//		if TRACING {
		//			_, span2 = s.tracer.StartContextSpan(sctx, "Cache.Put")
		//		}
		err := s.cachec.Put(key, &cacheproto.CacheInt{Val: int64(cnt)})
		//		if TRACING {
		//			//		err := s.cachec.PutTraced(tracing.SpanToContext(span2), key, &cacheproto.CacheInt{Val: int64(cnt)})
		//			span2.End()
		//		}
		if err != nil {
			return err
		}
	}

	inDate, _ := time.Parse(
		time.RFC3339,
		req.InDate+"T12:00:00+00:00")

	outDate, _ := time.Parse(
		time.RFC3339,
		req.OutDate+"T12:00:00+00:00")

	indate := inDate.String()[0:10]

	for inDate.Before(outDate) {
		inDate = inDate.AddDate(0, 0, 1)
		outdate := inDate.String()[0:10]

		q := fmt.Sprintf("INSERT INTO reservation (hotelid, customer, indate, outdate, number) VALUES ('%s', '%s', '%s', '%s', '%d');", hotelId, req.CustomerName, indate, outdate, req.Number)
		err := s.dbc.Exec(q)
		if err != nil {
			return fmt.Errorf("Insert failed %v", req)
		}
		indate = outdate
	}
	res.HotelIds = append(res.HotelIds, hotelId)
	return nil
}

func (s *Reserve) CheckAvailability(ctx fs.CtxI, req proto.ReserveRequest, res *proto.ReserveResult) error {
	var sctx context.Context
	//	var span trace.Span
	//	if TRACING {
	//		sctx, span = s.tracer.StartRPCSpan(&req, "CheckAvailability")
	//		defer span.End()
	//	} else {
	sctx = context.TODO()
	//	}

	hotelids := make([]string, 0)
	for _, hotelId := range req.HotelId {
		b, _, err := s.checkAvailability(sctx, hotelId, req)
		if err != nil {
			return err
		}
		if b {
			hotelids = append(hotelids, hotelId)
		}
	}
	res.HotelIds = hotelids
	return nil
}

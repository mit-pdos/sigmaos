package hotel

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	//	"go.opentelemetry.io/otel/trace"

	gproto "google.golang.org/protobuf/proto"

	"github.com/harlow/go-micro-services/data"

	"sigmaos/apps/cache"
	"sigmaos/apps/hotel/proto"
	dbclnt "sigmaos/db/clnt"
	db "sigmaos/debug"
	"sigmaos/api/fs"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/tracing"
	"sigmaos/util/perf"
)

var (
	sampleImg []byte
)

type RatePlans []*proto.RatePlan

func (r RatePlans) Len() int {
	return len(r)
}

func (r RatePlans) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r RatePlans) Less(i, j int) bool {
	return r[i].RoomType.TotalRate > r[j].RoomType.TotalRate
}

type Rate struct {
	dbc    *dbclnt.DbClnt
	cachec cache.CacheClnt
	tracer *tracing.Tracer
}

// Run starts the server
func RunRateSrv(job string, cache string) error {
	r := &Rate{}
	ssrv, err := sigmasrv.NewSigmaSrv(HOTELRATE, r, proc.GetProcEnv())
	if err != nil {
		return err
	}
	dbc, err := dbclnt.NewDbClnt(ssrv.MemFs.SigmaClnt().FsLib, sp.DBD)
	if err != nil {
		return err
	}
	r.dbc = dbc
	fsl, err := NewFsLib(HOTELRATE, ssrv.MemFs.SigmaClnt().GetDialProxyClnt())
	if err != nil {
		return err
	}
	cachec, err := NewCacheClnt(cache, fsl, job)
	if err != nil {
		return err
	}
	r.cachec = cachec

	file := data.MustAsset("data/inventory.json")
	rates := []*proto.RatePlan{}
	if err := json.Unmarshal(file, &rates); err != nil {
		return err
	}
	if err := r.initDB(rates); err != nil {
		return err
	}
	sampleImg = make([]byte, imgSizeMB*sp.MBYTE)
	for i := range sampleImg {
		sampleImg[i] = 'A'
	}
	//	r.tracer = tracing.Init("rate", proc.GetSigmaJaegerIP())
	//	defer r.tracer.Flush()
	p, err := perf.NewPerf(ssrv.MemFs.SigmaClnt().ProcEnv(), perf.HOTEL_RATE)
	if err != nil {
		db.DFatalf("NewPerf err %v\n", err)
	}
	defer p.Done()
	return ssrv.RunServer()
}

// GetRates gets rates for hotels
func (s *Rate) GetRates(ctx fs.CtxI, req proto.RateRequest, res *proto.RateResult) error {
	var sctx context.Context
	//	var span trace.Span
	//	if TRACING {
	//		sctx, span = s.tracer.StartRPCSpan(&req, "GetRates")
	//		defer span.End()
	//	} else {
	sctx = context.TODO()
	//	}

	ratePlans := make(RatePlans, 0)
	for _, hotelId := range req.HotelIds {
		r := &proto.RatePlan{}
		key := hotelId + "_rate"
		//		var span2 trace.Span
		//		if TRACING {
		//			_, span2 = s.tracer.StartContextSpan(sctx, "Cache.Get")
		//		}
		err := s.cachec.Get(key, r)
		//		if TRACING {
		//			//				err := s.cachec.GetTraced(tracing.SpanToContext(span2), key, r)
		//			span2.End()
		//		}
		if err != nil {
			if !cache.IsMiss(err) {
				return err
			}
			db.DPrintf(db.HOTEL_RATE, "Cache miss: key %v\n", hotelId)
			//			var span3 trace.Span
			//			if TRACING {
			//				_, span3 = s.tracer.StartContextSpan(sctx, "DB.GetRate")
			//			}
			r, err = s.getRate(sctx, hotelId)
			//			if TRACING {
			//				span3.End()
			//			}
			if err != nil {
				return err
			}

			b, err := gproto.Marshal(r)
			if err != nil {
				return err
			}
			db.DPrintf(db.ALWAYS, "Rate: Put %v bytes in cache for hotelID %v", len(b)+len(key), hotelId)

			//			var span4 trace.Span
			//			if TRACING {
			//				_, span4 = s.tracer.StartContextSpan(sctx, "Cache.Put")
			//			}
			err = s.cachec.Put(key, r)
			//			if TRACING {
			//				//			err = s.cachec.PutTraced(tracing.SpanToContext(span4), key, r)
			//				span4.End()
			//			}
			if err != nil {
				return err
			}
		}
		if r != nil && r.HotelId != "" {
			ratePlans = append(ratePlans, r)
		}
	}
	sort.Sort(ratePlans)
	res.RatePlans = ratePlans
	return nil
}

func (s *Rate) insertRate(r *proto.RatePlan) error {
	q := fmt.Sprintf("INSERT INTO rate (hotelid, code, indate, outdate, roombookrate, roomtotalrate, roomtotalinclusive, roomcode, roomcurrency, roomdescription) VALUES ('%s', '%s', '%s', '%s', '%f', '%f', '%f', '%s', '%s', '%s');", r.HotelId, r.Code, r.InDate, r.OutDate, r.RoomType.BookableRate, r.RoomType.TotalRate, r.RoomType.TotalRateInclusive, r.RoomType.Code, r.RoomType.Currency, r.RoomType.RoomDescription)
	if err := s.dbc.Exec(q); err != nil {
		return err
	}
	return nil
}

type RateFlat struct {
	HotelId                string
	Code                   string
	InDate                 string
	OutDate                string
	RoomBookableRate       float64
	RoomTotalRate          float64
	RoomTotalRateInclusive float64
	RoomCode               string
	RoomCurrency           string
	RoomDescription        string
}

func (s *Rate) getRate(sctx context.Context, id string) (*proto.RatePlan, error) {
	q := fmt.Sprintf("SELECT * from rate where hotelid='%s';", id)
	var rates []RateFlat
	//	var dbspan trace.Span
	//	if TRACING {
	//		_, dbspan = s.tracer.StartContextSpan(sctx, "db.Query")
	//	}
	error := s.dbc.Query(q, &rates)
	//	if TRACING {
	//		dbspan.End()
	//	}
	if error != nil {
		return nil, error
	}
	if len(rates) == 0 {
		return nil, nil
	}
	rf := &rates[0]
	r := &proto.RatePlan{
		HotelId: rf.HotelId,
		Code:    rf.Code,
		InDate:  rf.InDate,
		OutDate: rf.OutDate,
		RoomType: &proto.RoomType{
			BookableRate:       rf.RoomBookableRate,
			TotalRate:          rf.RoomTotalRate,
			TotalRateInclusive: rf.RoomTotalRateInclusive,
			Code:               rf.RoomCode,
			Currency:           rf.RoomCurrency,
			RoomDescription:    rf.RoomDescription,
			Image:              sampleImg,
		},
	}
	return r, nil
}

// loadRates loads rate codes from JSON file.
func (s *Rate) initDB(rates []*proto.RatePlan) error {
	q := fmt.Sprintf("truncate rate;")
	if err := s.dbc.Exec(q); err != nil {
		return err
	}
	for _, r := range rates {
		if err := s.insertRate(r); err != nil {
			return err
		}
	}
	for i := 7; i <= nhotel; i++ {
		if i%3 == 0 {
			end_date := "2015-04-"
			rate := 109.00
			rate_inc := 123.17
			if i%2 == 0 {
				end_date = end_date + "17"
			} else {
				end_date = end_date + "24"
			}

			if i%5 == 1 {
				rate = 120.00
				rate_inc = 140.00
			} else if i%5 == 2 {
				rate = 124.00
				rate_inc = 144.00
			} else if i%5 == 3 {
				rate = 132.00
				rate_inc = 158.00
			} else if i%5 == 4 {
				rate = 232.00
				rate_inc = 258.00
			}
			r := &proto.RatePlan{
				HotelId: strconv.Itoa(i),
				Code:    "RACK",
				InDate:  "2015-04-09",
				OutDate: end_date,
				RoomType: &proto.RoomType{
					BookableRate:       rate,
					TotalRate:          rate,
					TotalRateInclusive: rate_inc,
					Code:               "KNG",
					Currency:           "",
					RoomDescription:    "King sized bed",
				},
			}
			if err := s.insertRate(r); err != nil {
				return err
			}
		}
	}

	return nil
}

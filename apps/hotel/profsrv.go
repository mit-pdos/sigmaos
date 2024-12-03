package hotel

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	//	"go.opentelemetry.io/otel/trace"

	"github.com/harlow/go-micro-services/data"

	"sigmaos/apps/cache"
	"sigmaos/apps/hotel/proto"
	dbclnt "sigmaos/proxy/db/clnt"
	db "sigmaos/debug"
	"sigmaos/api/fs"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/tracing"
)

type ProfSrv struct {
	dbc    *dbclnt.DbClnt
	cachec cache.CacheClnt
	tracer *tracing.Tracer
}

func RunProfSrv(job string, cache string) error {
	ps := &ProfSrv{}
	ssrv, err := sigmasrv.NewSigmaSrv(HOTELPROF, ps, proc.GetProcEnv())
	if err != nil {
		return err
	}
	dbc, err := dbclnt.NewDbClnt(ssrv.MemFs.SigmaClnt().FsLib, sp.DBD)
	if err != nil {
		return err
	}
	ps.dbc = dbc
	fsl, err := NewFsLib(HOTELPROF, ssrv.MemFs.SigmaClnt().GetDialProxyClnt())
	if err != nil {
		return err
	}
	cachec, err := NewCacheClnt(cache, fsl, job)
	if err != nil {
		return err
	}
	ps.cachec = cachec
	file := data.MustAsset("data/hotels.json")
	profs := []*Profile{}
	if err := json.Unmarshal(file, &profs); err != nil {
		return err
	}
	ps.initDB(profs)
	//	ps.tracer = tracing.Init("prof", proc.GetSigmaJaegerIP())
	//	defer ps.tracer.Flush()
	return ssrv.RunServer()
}

// Inserts a flatten profile into db
func (ps *ProfSrv) insertProf(p *Profile) error {
	q := fmt.Sprintf("INSERT INTO profile (hotelid, name, phone, description, streetnumber, streetname, city, state, country, postal, lat, lon) VALUES ('%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%f', '%f');", p.Id, p.Name, p.PhoneNumber, p.Description, p.Address.StreetNumber, p.Address.StreetName, p.Address.City, p.Address.State, p.Address.Country, p.Address.PostalCode, p.Address.Lat, p.Address.Lon)
	if err := ps.dbc.Exec(q); err != nil {
		return err
	}
	return nil
}

func (ps *ProfSrv) getProf(sctx context.Context, id string) (*proto.ProfileFlat, error) {
	q := fmt.Sprintf("SELECT * from profile where hotelid='%s';", id)
	var profs []proto.ProfileFlat

	//	var dbspan trace.Span
	//	if TRACING {
	//		_, dbspan = ps.tracer.StartContextSpan(sctx, "db.Query")
	//	}
	error := ps.dbc.Query(q, &profs)
	//	if TRACING {
	//		dbspan.End()
	//	}
	if error != nil {
		return nil, error
	}
	if len(profs) == 0 {
		return nil, fmt.Errorf("unknown hotel %s", id)
	}
	return &profs[0], nil
}

func (ps *ProfSrv) initDB(profs []*Profile) error {
	q := fmt.Sprintf("truncate profile;")
	if err := ps.dbc.Exec(q); err != nil {
		return err
	}
	for _, p := range profs {
		if err := ps.insertProf(p); err != nil {
			return err
		}
	}

	for i := 7; i <= nhotel; i++ {
		p := Profile{
			strconv.Itoa(i),
			"St. Regis San Francisco",
			"(415) 284-40" + strconv.Itoa(i),
			"St. Regis Museum Tower is a 42-story, 484 ft skyscraper in the South of Market district of San Francisco, California, adjacent to Yerba Buena Gardens, Moscone Center, PacBell Building and the San Francisco Museum of Modern Art.",
			&Address{
				"125",
				"3rd St",
				"San Francisco",
				"CA",
				"United States",
				"94109",
				37.7835 + float32(i)/500.0*3,
				-122.41 + float32(i)/500.0*4,
			},
			nil,
		}
		if err := ps.insertProf(&p); err != nil {
			return err
		}
	}

	return nil
}

func (ps *ProfSrv) GetProfiles(ctx fs.CtxI, req proto.ProfRequest, res *proto.ProfResult) error {
	var sctx context.Context
	//	var span trace.Span
	//	if TRACING {
	//		sctx, span = ps.tracer.StartRPCSpan(&req, "GetProfiles")
	//		defer span.End()
	//	} else {
	sctx = context.TODO()
	//}

	db.DPrintf(db.HOTEL_PROF, "Req %v\n", req)
	for _, id := range req.HotelIds {
		p := &proto.ProfileFlat{}
		key := id + "_prof"
		//		var span2 trace.Span
		//		if TRACING {
		//			_, span2 = ps.tracer.StartContextSpan(sctx, "Cache.Get")
		//		}
		err := ps.cachec.Get(key, p)
		//		if TRACING {
		//			//		err := ps.cachec.GetTraced(tracing.SpanToContext(span2), key, p)
		//			span2.End()
		//		}
		if err != nil {
			if !cache.IsMiss(err) {
				return err
			}
			db.DPrintf(db.HOTEL_PROF, "Cache miss: key %v\n", id)
			p, err = ps.getProf(sctx, id)
			if err != nil {
				return err
			}
			//			var span3 trace.Span
			//			if TRACING {
			//				_, span3 = ps.tracer.StartContextSpan(sctx, "Cache.Put")
			//			}
			err = ps.cachec.Put(key, p)
			//			if TRACING {
			//				//			err = ps.cachec.PutTraced(tracing.SpanToContext(span3), key, p)
			//				span3.End()
			//			}
			if err != nil {
				return err
			}
		}
		if p != nil && p.HotelId != "" {
			res.Hotels = append(res.Hotels, p)
		}
	}
	return nil
}

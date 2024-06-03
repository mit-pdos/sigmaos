package hotel

import (
	"encoding/json"
	"log"
	"math"
	"strconv"

	//	"go.opentelemetry.io/otel/trace"

	"github.com/hailocab/go-geoindex"
	"github.com/harlow/go-micro-services/data"
	// "github.com/harlow/go-micro-services/internal/proto/geo"

	"sigmaos/fs"
	"sigmaos/hotel/proto"
	"sigmaos/proc"
	"sigmaos/sigmasrv"
	"sigmaos/tracing"
)

//	type RecRequest struct {
//		Require string
//		Lat     float64
//		Lon     float64
//	}
//
//	type RecResult struct {
//		HotelIds []string
//	}

type Hotel struct {
	HId    string
	HLat   float64
	HLon   float64
	HRate  float64
	HPrice float64
}

type Rec struct {
	hotels map[string]*Hotel
	tracer *tracing.Tracer
}

// Run starts the server
func RunRecSrv(n string) error {
	r := &Rec{}
	r.hotels = loadRecTable("data/hotels.json")
	ssrv, err := sigmasrv.NewSigmaSrv(HOTELREC, r, proc.GetProcEnv())
	if err != nil {
		return err
	}
	//	r.tracer = tracing.Init("rec", proc.GetSigmaJaegerIP())
	//	defer r.tracer.Flush()
	return ssrv.RunServer()
}

// GiveRecommendation returns recommendations within a given requirement.
func (s *Rec) GetRecs(ctx fs.CtxI, req proto.RecRequest, res *proto.RecResult) error {
	//	var span trace.Span
	//	if TRACING {
	//		_, span = s.tracer.StartRPCSpan(&req, "GetRecs")
	//		defer span.End()
	//	}

	require := req.Require
	if require == "dis" {
		p1 := &geoindex.GeoPoint{
			Pid:  "",
			Plat: req.Lat,
			Plon: req.Lon,
		}
		min := math.MaxFloat64
		for _, hotel := range s.hotels {
			tmp := float64(geoindex.Distance(p1, &geoindex.GeoPoint{
				Pid:  "",
				Plat: hotel.HLat,
				Plon: hotel.HLon,
			})) / 1000
			if tmp < min {
				min = tmp
			}
		}
		for _, hotel := range s.hotels {
			tmp := float64(geoindex.Distance(p1, &geoindex.GeoPoint{
				Pid:  "",
				Plat: hotel.HLat,
				Plon: hotel.HLon,
			})) / 1000
			if tmp == min {
				res.HotelIds = append(res.HotelIds, hotel.HId)
			}
		}
	} else if require == "rate" {
		max := 0.0
		for _, hotel := range s.hotels {
			if hotel.HRate > max {
				max = hotel.HRate
			}
		}
		for _, hotel := range s.hotels {
			if hotel.HRate == max {
				res.HotelIds = append(res.HotelIds, hotel.HId)
			}
		}
	} else if require == "price" {
		min := math.MaxFloat64
		for _, hotel := range s.hotels {
			if hotel.HPrice < min {
				min = hotel.HPrice
			}
		}
		for _, hotel := range s.hotels {
			if hotel.HPrice == min {
				res.HotelIds = append(res.HotelIds, hotel.HId)
			}
		}
	} else {
		// log.Warn().Msgf("Wrong require parameter: %v", require)
	}

	return nil
}

type Profile struct {
	Id          string
	Name        string
	PhoneNumber string
	Description string
	Address     *Address
	Images      []*Image
}

type Address struct {
	StreetNumber string
	StreetName   string
	City         string
	State        string
	Country      string
	PostalCode   string
	Lat          float32
	Lon          float32
}

type Image struct {
	Url     string
	Default bool
}

// DeathStarBench uses some of the profile data from microservices
func loadRecTable(path string) map[string]*Hotel {
	rates := [6]float64{109.00, 139.00, 109.00, 129.00, 119.00, 149.00}
	prices := [6]float64{150.00, 120.00, 190.00, 160.00, 140.00, 200.00}

	file := data.MustAsset(path)
	profs := []*Profile{}
	if err := json.Unmarshal(file, &profs); err != nil {
		log.Fatalf("Failed to load json: %v", err)
	}

	hs := make(map[string]*Hotel)
	for i, p := range profs {
		h := &Hotel{}
		h.HId = p.Id
		h.HLat = float64(p.Address.Lat)
		h.HLon = float64(p.Address.Lon)
		h.HRate = rates[i]
		h.HPrice = prices[i]
		hs[h.HId] = h
	}
	for i := 7; i <= nhotel; i++ {
		hotel_id := strconv.Itoa(i)
		lat := 37.7835 + float64(i)/500.0*3
		lon := -122.41 + float64(i)/500.0*4
		rate := 135.00
		rate_inc := 179.00
		if i%3 == 0 {
			if i%5 == 0 {
				rate = 109.00
				rate_inc = 123.17
			} else if i%5 == 1 {
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
		}
		h := &Hotel{
			HId:    hotel_id,
			HLat:   lat,
			HLon:   lon,
			HRate:  rate,
			HPrice: rate_inc,
		}
		hs[h.HId] = h
	}
	return hs
}

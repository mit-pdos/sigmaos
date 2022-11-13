package hotel

import (
	"encoding/json"
	"log"
	"math"

	"github.com/hailocab/go-geoindex"
	"github.com/harlow/go-micro-services/data"
	// "github.com/harlow/go-micro-services/internal/proto/geo"

	"sigmaos/fs"
	np "sigmaos/ninep"
	"sigmaos/protdevsrv"
)

type RecRequest struct {
	Require string
	Lat     float64
	Lon     float64
}

type RecResult struct {
	HotelIds []string
}

type Hotel struct {
	HId    string
	HLat   float64
	HLon   float64
	HRate  float64
	HPrice float64
}

type Rec struct {
	hotels map[string]*Hotel
}

// Run starts the server
func RunRecSrv() error {
	r := &Rec{}
	r.hotels = loadRecTable("data/hotels.json")
	protdevsrv.Run(np.HOTELREC, r.mkStream)
	return nil
}

type StreamRec struct {
	rep []byte
	rec *Rec
}

func (rec *Rec) mkStream() (fs.File, *np.Err) {
	st := &StreamRec{}
	st.rec = rec
	return st, nil
}

// XXX wait on close before processing data?
func (st *StreamRec) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	var args RecRequest
	err := json.Unmarshal(b, &args)
	log.Printf("recs %v\n", args)
	res, err := st.rec.GetRecs(&args)
	if err != nil {
		return 0, np.MkErrError(err)
	}
	st.rep, err = json.Marshal(res)
	if err != nil {
		return 0, np.MkErrError(err)
	}
	return np.Tsize(len(b)), nil
}

// XXX incremental read
func (st *StreamRec) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	if len(st.rep) == 0 || off > 0 {
		return nil, nil
	}
	return st.rep, nil
}

// GiveRecommendation returns recommendations within a given requirement.
func (s *Rec) GetRecs(req *RecRequest) (*RecResult, error) {
	res := new(RecResult)
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

	return res, nil
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
		log.Printf("hotel: %v\n", h)
	}
	return hs
}

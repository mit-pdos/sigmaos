package hotel

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/harlow/go-micro-services/data"

	"sigmaos/cacheclnt"
	"sigmaos/dbclnt"
	db "sigmaos/debug"
	np "sigmaos/ninep"
	"sigmaos/protdevsrv"
)

type RateRequest struct {
	HotelIds []string
	InDate   string
	OutDate  string
}

type RoomType struct {
	BookableRate       float64
	TotalRate          float64
	TotalRateInclusive float64
	Code               string
	Currency           string
	RoomDescription    string
}

type RatePlan struct {
	HotelId  string
	Code     string
	InDate   string
	OutDate  string
	RoomType *RoomType
}

type RatePlans []*RatePlan

func (r RatePlans) Len() int {
	return len(r)
}

func (r RatePlans) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r RatePlans) Less(i, j int) bool {
	return r[i].RoomType.TotalRate > r[j].RoomType.TotalRate
}

type RateResult struct {
	RatePlans []*RatePlan
}

type Rate struct {
	dbc    *dbclnt.DbClnt
	cachec *cacheclnt.CacheClnt
}

// Run starts the server
func RunRateSrv(n string) error {
	r := &Rate{}
	pds, err := protdevsrv.MakeProtDevSrv(np.HOTELRATE, r)
	if err != nil {
		return err
	}
	dbc, err := dbclnt.MkDbClnt(pds.MemFs.FsLib(), np.DBD)
	if err != nil {
		return err
	}
	r.dbc = dbc
	cachec, err := cacheclnt.MkCacheClnt(pds.MemFs.FsLib(), NCACHE)
	if err != nil {
		return err
	}
	r.cachec = cachec

	file := data.MustAsset("data/inventory.json")
	rates := []*RatePlan{}
	if err := json.Unmarshal(file, &rates); err != nil {
		return err
	}
	if err := r.initDB(rates); err != nil {
		return err
	}
	return pds.RunServer()
}

// GetRates gets rates for hotels
func (s *Rate) GetRates(req RateRequest, res *RateResult) error {
	ratePlans := make(RatePlans, 0)
	for _, hotelId := range req.HotelIds {
		r := &RatePlan{}
		key := hotelId + "_rate"
		if err := s.cachec.Get(key, r); err != nil {
			if err.Error() != cacheclnt.ErrMiss.Error() {
				return err
			}
			db.DPrintf("HOTELRATE", "Cache miss: key %v\n", hotelId)
			r, err = s.getRate(hotelId)
			if err != nil {
				return err
			}
			if err := s.cachec.Set(key, r); err != nil {
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

func (s *Rate) insertRate(r *RatePlan) error {
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

func (s *Rate) getRate(id string) (*RatePlan, error) {
	q := fmt.Sprintf("SELECT * from rate where hotelid='%s';", id)
	var rates []RateFlat
	if error := s.dbc.Query(q, &rates); error != nil {
		return nil, error
	}
	if len(rates) == 0 {
		return nil, nil
	}
	rf := &rates[0]
	r := &RatePlan{
		HotelId: rf.HotelId,
		Code:    rf.Code,
		InDate:  rf.InDate,
		OutDate: rf.OutDate,
		RoomType: &RoomType{
			rf.RoomBookableRate,
			rf.RoomTotalRate,
			rf.RoomTotalRateInclusive,
			rf.RoomCode,
			rf.RoomCurrency,
			rf.RoomDescription,
		},
	}
	return r, nil
}

// loadRates loads rate codes from JSON file.
func (s *Rate) initDB(rates []*RatePlan) error {
	q := fmt.Sprintf("truncate rate;")
	if err := s.dbc.Exec(q); err != nil {
		return err
	}
	for _, r := range rates {
		if err := s.insertRate(r); err != nil {
			return err
		}
	}
	for i := 7; i <= NHOTEL; i++ {
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
			r := &RatePlan{
				HotelId: strconv.Itoa(i),
				Code:    "RACK",
				InDate:  "2015-04-09",
				OutDate: end_date,
				RoomType: &RoomType{
					rate,
					rate,
					rate_inc,
					"KNG",
					"",
					"King sized bed",
				},
			}
			if err := s.insertRate(r); err != nil {
				return err
			}
		}
	}

	return nil
}

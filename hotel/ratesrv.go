package hotel

import (
	"encoding/json"
	"log"
	"sort"
	"strconv"

	"github.com/harlow/go-micro-services/data"

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
	rateTable map[string]*RatePlan
}

// Run starts the server
func RunRateSrv(n string) error {
	r := &Rate{}
	r.rateTable = loadRateTable("data/inventory.json")
	pds := protdevsrv.MakeProtDevSrv(np.HOTELRATE, r)
	return pds.RunServer()
}

// GetRates gets rates for hotels
func (s *Rate) GetRates(req RateRequest, res *RateResult) error {
	ratePlans := make(RatePlans, 0)
	for _, hotelID := range req.HotelIds {
		if s.rateTable[hotelID] != nil {
			ratePlans = append(ratePlans, s.rateTable[hotelID])
		}
	}
	sort.Sort(ratePlans)
	res.RatePlans = ratePlans
	return nil
}

// loadRates loads rate codes from JSON file.
func loadRateTable(path string) map[string]*RatePlan {
	file := data.MustAsset(path)

	rates := []*RatePlan{}
	if err := json.Unmarshal(file, &rates); err != nil {
		log.Fatalf("Failed to load json: %v", err)
	}

	rateTable := make(map[string]*RatePlan)
	for _, ratePlan := range rates {
		rateTable[ratePlan.HotelId] = ratePlan
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
			rateTable[strconv.Itoa(i)] = r
		}
	}

	return rateTable
}

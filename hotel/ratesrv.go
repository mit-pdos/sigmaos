package hotel

import (
	"encoding/json"
	"log"

	"github.com/harlow/go-micro-services/data"
	// "github.com/harlow/go-micro-services/internal/proto/geo"

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
	InDate   string
	OutDate  string
	RoomType *RoomType
}

type RateResult struct {
	RatePlans []*RatePlan
}

type Rate struct {
	rateTable map[stay]*RatePlan
}

// Run starts the server
func RunRateSrv(n string) error {
	r := &Rate{}
	r.rateTable = loadRateTable("data/inventory.json")
	pds := protdevsrv.MakeProtDevSrv(np.HOTELRATE, r)
	return pds.RunServer()
}

// GetRates gets rates for hotels for specific date range.
func (s *Rate) GetRates(req RateRequest, res *RateResult) error {
	for _, hotelID := range req.HotelIds {
		stay := stay{
			HotelID: hotelID,
			InDate:  req.InDate,
			OutDate: req.OutDate,
		}
		if s.rateTable[stay] != nil {
			res.RatePlans = append(res.RatePlans, s.rateTable[stay])
		}
	}

	return nil
}

// loadRates loads rate codes from JSON file.
func loadRateTable(path string) map[stay]*RatePlan {
	file := data.MustAsset(path)

	rates := []*RatePlan{}
	if err := json.Unmarshal(file, &rates); err != nil {
		log.Fatalf("Failed to load json: %v", err)
	}

	rateTable := make(map[stay]*RatePlan)
	for _, ratePlan := range rates {
		stay := stay{
			HotelID: ratePlan.HotelId,
			InDate:  ratePlan.InDate,
			OutDate: ratePlan.OutDate,
		}
		rateTable[stay] = ratePlan
	}

	return rateTable
}

type stay struct {
	HotelID string
	InDate  string
	OutDate string
}

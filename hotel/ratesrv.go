package hotel

import (
	"encoding/json"
	"log"

	"github.com/harlow/go-micro-services/data"
	// "github.com/harlow/go-micro-services/internal/proto/geo"

	"sigmaos/fs"
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
func RunRateSrv() error {
	r := &Rate{}
	r.rateTable = loadRateTable("data/inventory.json")
	protdevsrv.Run(np.HOTELRATE, r.mkStream)
	return nil
}

type StreamRate struct {
	rep  []byte
	rate *Rate
}

func (rate *Rate) mkStream() (fs.File, *np.Err) {
	st := &StreamRate{}
	st.rate = rate
	return st, nil
}

// XXX wait on close before processing data?
func (st *StreamRate) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	var args RateRequest
	err := json.Unmarshal(b, &args)
	log.Printf("rate %v\n", args)
	res, err := st.rate.GetRates(&args)
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
func (st *StreamRate) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	if len(st.rep) == 0 || off > 0 {
		return nil, nil
	}
	return st.rep, nil
}

// GetRates gets rates for hotels for specific date range.
func (s *Rate) GetRates(req *RateRequest) (*RateResult, error) {
	res := new(RateResult)

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

	return res, nil
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

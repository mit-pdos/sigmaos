package hotel

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/hotel/proto"
	rpcclnt "sigmaos/rpc/clnt"
)

func RandSearchReq(wc *WebClnt, r *rand.Rand) error {
	in_date := r.Intn(14) + 9
	out_date := in_date + r.Intn(5) + 1
	in_date_str := fmt.Sprintf("2015-04-%d", in_date)
	if in_date <= 9 {
		in_date_str = fmt.Sprintf("2015-04-0%d", in_date)
	}
	out_date_str := fmt.Sprintf("2015-04-%d", out_date)
	if out_date <= 9 {
		out_date_str = fmt.Sprintf("2015-04-0%d", out_date)
	}
	lat := 38.0235 + (float64(r.Intn(481))-240.5)/1000.0
	lon := -122.095 + (float64(r.Intn(325))-157.0)/1000.0
	//	lat := 38.0235 + (float64(r.Intn(481*nhotel/80))-240.5)/1000.0
	//	lon := -122.095 + (float64(r.Intn(325*nhotel/80))-157.0)/1000.0
	return wc.Search(in_date_str, out_date_str, lat, lon)
}

func RandRecsReq(wc *WebClnt, r *rand.Rand) error {
	coin := toss(r)
	req := ""
	if coin < 0.33 {
		req = "dis"
	} else if coin < 0.66 {
		req = "rate"
	} else {
		req = "price"
	}
	lat := 38.0235 + (float64(r.Intn(481))-240.5)/1000.0
	lon := -122.095 + (float64(r.Intn(325))-157.0)/1000.0
	return wc.Recs(req, lat, lon)
}

func RandLoginReq(wc *WebClnt, r *rand.Rand) (string, error) {
	suffix := strconv.Itoa(r.Intn(500))
	user := "Cornell_" + suffix
	pw := NewPassword(suffix)
	return wc.Login(user, pw)
}

func RandReserveReq(wc *WebClnt, r *rand.Rand) (string, error) {
	userID := r.Intn(500)
	return RandReserveReqUser(wc, r, userID)
}

func RandReserveReqUser(wc *WebClnt, r *rand.Rand, userID int) (string, error) {
	in_date := r.Intn(14) + 9
	out_date := in_date + r.Intn(5) + 1
	in_date_str := fmt.Sprintf("2015-04-%d", in_date)
	if in_date <= 9 {
		in_date_str = fmt.Sprintf("2015-04-0%d", in_date)
	}
	out_date_str := fmt.Sprintf("2015-04-%d", out_date)
	if out_date <= 9 {
		out_date_str = fmt.Sprintf("2015-04-0%d", out_date)
	}
	hotelid := strconv.Itoa(r.Intn(nhotel) + 1)
	suffix := strconv.Itoa(userID)
	user := "Cornell_" + suffix
	pw := NewPassword(suffix)
	cust_name := user
	num := 1
	lat := 38.0235 + (float64(r.Intn(481))-240.5)/1000.0
	lon := -122.095 + (float64(r.Intn(325))-157.0)/1000.0
	return wc.Reserve(in_date_str, out_date_str, lat, lon, hotelid, user, cust_name, pw, num)
}

func RandCheckAvailabilityReq(rpcc *rpcclnt.RPCClnt, r *rand.Rand) error {
	in_date := r.Intn(14) + 9
	out_date := in_date + r.Intn(5) + 1
	in_date_str := fmt.Sprintf("2015-04-%d", in_date)
	if in_date <= 9 {
		in_date_str = fmt.Sprintf("2015-04-0%d", in_date)
	}
	out_date_str := fmt.Sprintf("2015-04-%d", out_date)
	if out_date <= 9 {
		out_date_str = fmt.Sprintf("2015-04-0%d", out_date)
	}
	nids := rand.Intn(5)
	ids := make([]string, 0, nids)
	for i := 0; i < nids; i++ {
		ids = append(ids, strconv.Itoa(rand.Intn(nhotel-7)+7))
	}
	arg := &proto.ReserveReq{
		HotelId:      ids,
		CustomerName: "Cornell_0",
		InDate:       in_date_str,
		OutDate:      out_date_str,
		Number:       1,
	}
	var res proto.ReserveRep
	return rpcc.RPC("Reserve.CheckAvailability", arg, &res)
}

func SpecificGeoReq(wc *WebClnt, lat float64, lon float64) (string, error) {
	return wc.Geo(lat, lon)
}

func GeoReq(wc *WebClnt) (string, error) {
	// Same coordinates to make sure performance is very consistent.
	lat := 37.7749
	lon := -122.4194
	return wc.Geo(lat, lon)
}

func toss(r *rand.Rand) float64 {
	toss := r.Intn(1000)
	return float64(toss) / 1000
}

func RunDSB(t *testing.T, N int, wc *WebClnt, r *rand.Rand) {
	const (
		search_ratio    = 0.6
		recommend_ratio = 0.39
		user_ratio      = 0.005
		reserve_ratio   = 0.005
	)
	for i := 0; i < N; i++ {
		coin := toss(r)
		if coin < search_ratio {
			err := RandSearchReq(wc, r)
			assert.Nil(t, err, "Err %v", err)
		} else if coin < search_ratio+recommend_ratio {
			err := RandRecsReq(wc, r)
			assert.Nil(t, err, "Err %v", err)
		} else if coin < search_ratio+recommend_ratio+user_ratio {
			_, err := RandLoginReq(wc, r)
			assert.Nil(t, err, "Err %v", err)
		} else {
			_, err := RandReserveReq(wc, r)
			assert.Nil(t, err, "Err %v", err)
		}
	}
}

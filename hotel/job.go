package hotel

import (
	"fmt"
	"path"
	"strconv"

	"math/rand"
	db "sigmaos/debug"
	"sigmaos/fslib"
)

const (
	HOTEL      = "hotel"
	HOTELDIR   = "name/hotel/"
	MEMFS      = "memfs"
	HTTP_ADDRS = "http-addr"
	NCACHE     = 3
)

func JobDir(job string) string {
	return path.Join(HOTELDIR, job)
}

func JobHTTPAddrsPath(job string) string {
	return path.Join(JobDir(job), HTTP_ADDRS)
}

func MemFsPath(job string) string {
	return path.Join(JobDir(job), MEMFS)
}

func GetJobHTTPAddrs(fsl *fslib.FsLib, job string) ([]string, error) {
	p := JobHTTPAddrsPath(job)
	var addrs []string
	err := fsl.GetFileJson(p, &addrs)
	return addrs, err
}

func InitHotelFs(fsl *fslib.FsLib, jobname string) {
	fsl.MkDir(HOTELDIR, 0777)
	if err := fsl.MkDir(JobDir(jobname), 0777); err != nil {
		db.DFatalf("Mkdir %v err %v\n", JobDir(jobname), err)
	}
}

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
	pw := MkPassword(suffix)
	return wc.Login(user, pw)
}

func RandReserveReq(wc *WebClnt, r *rand.Rand) (string, error) {
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
	hotelid := strconv.Itoa(r.Intn(80) + 1)
	suffix := strconv.Itoa(r.Intn(500))
	user := "Cornell_" + suffix
	pw := MkPassword(suffix)
	cust_name := user
	num := 1
	lat := 38.0235 + (float64(r.Intn(481))-240.5)/1000.0
	lon := -122.095 + (float64(r.Intn(325))-157.0)/1000.0
	return wc.Reserve(in_date_str, out_date_str, lat, lon, hotelid, user, cust_name, pw, num)
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

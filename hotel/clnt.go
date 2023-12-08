package hotel

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/netsigma"
	sp "sigmaos/sigmap"
)

type WebClnt struct {
	jobname  string
	srvaddrs sp.Taddrs
	baseurl  string
	clnt     *http.Client
	*fslib.FsLib
}

func NewWebClnt(fsl *fslib.FsLib, job string) *WebClnt {
	addrs, err := GetJobHTTPAddrs(fsl, job)
	if err != nil {
		db.DFatalf("Error wwwd job http addrs: %v", err)
	}
	//	transport := &http.Transport{
	//		Dial: (&net.Dialer{
	//			Timeout: 2 * time.Minute,
	//		}).Dial,
	//	}
	clnt := &http.Client{
		Timeout:   2 * time.Minute,
		Transport: http.DefaultTransport,
	}
	// XXX This is sort of arbitrary, perhaps change or remove?.
	clnt.Transport.(*http.Transport).MaxIdleConnsPerHost = 10000
	addrs = netsigma.Rearrange(sp.ROOTREALM.String(), addrs)
	db.DPrintf(db.ALWAYS, "Advertised addr %v", addrs[0].Addr)
	return &WebClnt{job, addrs, "http://" + addrs[0].Addr, clnt, fsl}
}

func (wc *WebClnt) request(path string, vals url.Values) ([]byte, error) {
	u, err := url.Parse(wc.baseurl + path)
	if err != nil {
		return nil, err
	}
	u.RawQuery, err = url.QueryUnescape(vals.Encode())
	if err != nil {
		return nil, err
	}
	resp, err := wc.clnt.Get(u.String())
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code %v body %s", resp.StatusCode, body)
	}
	return body, nil
}

func (wc *WebClnt) Login(u, p string) (string, error) {
	vals := url.Values{}
	vals.Set("username", u)
	vals.Set("password", p)
	db.DPrintf(db.HOTEL_CLNT, "Login vals %v\n", vals)
	body, err := wc.request("/user", vals)
	if err != nil {
		return "", err
	}
	repl := make(map[string]interface{})
	err = json.Unmarshal(body, &repl)
	if err != nil {
		return "", err
	}
	return repl["message"].(string), nil
}

func (wc *WebClnt) Search(inDate, outDate string, lat, lon float64) error {
	vals := url.Values{}
	vals.Set("inDate", inDate)
	vals.Set("outDate", outDate)
	vals.Set("lat", strconv.FormatFloat(lat, 'f', -1, 64))
	vals.Set("lon", strconv.FormatFloat(lon, 'f', -1, 64))
	db.DPrintf(db.HOTEL_CLNT, "Search vals %v\n", vals)
	_, err := wc.request("/hotels", vals)
	if err != nil {
		return err
	}
	return nil
}

func (wc *WebClnt) Recs(require string, lat, lon float64) error {
	vals := url.Values{}
	vals.Set("require", require)
	vals.Add("lat", strconv.FormatFloat(lat, 'f', -1, 64))
	vals.Add("lon", strconv.FormatFloat(lon, 'f', -1, 64))
	db.DPrintf(db.HOTEL_CLNT, "Recs vals %v\n", vals)
	_, err := wc.request("/recommendations", vals)
	if err != nil {
		return err
	}
	return nil
}

func (wc *WebClnt) Reserve(inDate, outDate string, lat, lon float64, hotelid, name, u, p string, n int) (string, error) {
	vals := url.Values{}
	vals.Set("inDate", inDate)
	vals.Set("outDate", outDate)
	vals.Set("lat", strconv.FormatFloat(lat, 'f', -1, 64))
	vals.Set("lon", strconv.FormatFloat(lon, 'f', -1, 64))
	vals.Set("hotelId", hotelid)
	vals.Set("customerName", name)
	vals.Set("username", u)
	vals.Set("password", p)
	vals.Set("number", strconv.Itoa(n))

	db.DPrintf(db.HOTEL_CLNT, "Reserve vals %v\n", vals)

	body, err := wc.request("/reservation", vals)
	if err != nil {
		return "", err
	}
	repl := make(map[string]interface{})
	err = json.Unmarshal(body, &repl)
	if err != nil {
		return "", err
	}
	return repl["message"].(string), nil
}

func (wc *WebClnt) Geo(lat, lon float64) (string, error) {
	vals := url.Values{}
	vals.Set("lat", strconv.FormatFloat(lat, 'f', -1, 64))
	vals.Set("lon", strconv.FormatFloat(lon, 'f', -1, 64))
	db.DPrintf(db.HOTEL_CLNT, "Geo vals %v\n", vals)
	body, err := wc.request("/geo", vals)
	if err != nil {
		return "", err
	}
	repl := make(map[string]interface{})
	err = json.Unmarshal(body, &repl)
	if err != nil {
		return "", err
	}
	return repl["message"].(string), nil
}

func (wc *WebClnt) SaveResults() (string, error) {
	vals := url.Values{}
	body, err := wc.request("/saveresults", vals)
	if err != nil {
		return "", err
	}
	repl := make(map[string]interface{})
	err = json.Unmarshal(body, &repl)
	if err != nil {
		return "", err
	}
	return repl["message"].(string), nil
}

func (wc *WebClnt) StartRecording() (string, error) {
	vals := url.Values{}
	body, err := wc.request("/startrecording", vals)
	if err != nil {
		return "", err
	}
	repl := make(map[string]interface{})
	err = json.Unmarshal(body, &repl)
	if err != nil {
		return "", err
	}
	return repl["message"].(string), nil
}

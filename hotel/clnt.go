package hotel

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	db "sigmaos/debug"
	"sigmaos/fslib"
)

type WebClnt struct {
	jobname  string
	srvaddrs []string
	baseurl  string
	clnt     *http.Client
	*fslib.FsLib
}

func MakeWebClnt(fsl *fslib.FsLib, job string) *WebClnt {
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
	return &WebClnt{job, addrs, "http://" + addrs[0], clnt, fsl}
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
		return nil, fmt.Errorf("%v %s", resp.StatusCode, body)
	}
	return body, nil
}

func (wc *WebClnt) Login(u, p string) (string, error) {
	vals := url.Values{}
	vals.Set("username", u)
	vals.Set("password", p)
	db.DPrintf("WEBC", "Login vals %v\n", vals)
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
	vals.Set("lat", fmt.Sprintf("%f", lat))
	vals.Set("lon", fmt.Sprintf("%f", lon))
	db.DPrintf("WEBC", "Search vals %v\n", vals)
	_, err := wc.request("/hotels", vals)
	if err != nil {
		return err
	}
	return nil
}

func (wc *WebClnt) Recs(require string, lat, lon float64) error {
	vals := url.Values{}
	vals.Set("require", require)
	vals.Add("lat", fmt.Sprintf("%f", lat))
	vals.Add("lon", fmt.Sprintf("%f", lon))
	db.DPrintf("WEBC", "Recs vals %v\n", vals)
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
	vals.Set("lat", fmt.Sprintf("%f", lat))
	vals.Set("lon", fmt.Sprintf("%f", lon))
	vals.Set("hotelId", hotelid)
	vals.Set("customerName", name)
	vals.Set("username", u)
	vals.Set("password", p)
	vals.Set("number", fmt.Sprintf("%d", n))

	db.DPrintf("WEBC", "Reserve vals %v\n", vals)

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
	vals.Set("lat", fmt.Sprintf("%f", lat))
	vals.Set("lon", fmt.Sprintf("%f", lon))
	db.DPrintf("WEBC", "Geo vals %v\n", vals)
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

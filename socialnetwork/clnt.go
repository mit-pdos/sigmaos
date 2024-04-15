package socialnetwork

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	//"strconv"
	dbg "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/netsigma"
	sp "sigmaos/sigmap"
	"time"
)

type WebClnt struct {
	jobname  string
	srvaddrs sp.Taddrs
	baseurl  string
	clnt     *http.Client
	*fslib.FsLib
}

func NewWebClnt(fsl *fslib.FsLib, job string) *WebClnt {
	feAddrs, err := GetJobHTTPAddrs(fsl, job)
	if err != nil {
		dbg.DFatalf("Error wwwd job http addrs: %v", err)
	}
	return NewWebClntWithAddr(fsl, job, feAddrs)
}

func NewWebClntWithAddr(fsl *fslib.FsLib, job string, feAddrs sp.Taddrs) *WebClnt {
	clnt := &http.Client{
		Timeout:   1 * time.Minute,
		Transport: http.DefaultTransport,
	}
	// XXX This is sort of arbitrary, perhaps change or remove?.
	clnt.Transport.(*http.Transport).MaxIdleConnsPerHost = 10000
	addrs := netsigma.Rearrange(sp.ROOTREALM.String(), feAddrs)
	dbg.DPrintf(dbg.TEST, "Advertised addrs %v", addrs)
	dbg.DPrintf(dbg.SOCIAL_NETWORK_CLNT, "Advertised addr %v", addrs[0])
	return &WebClnt{job, addrs, "http://" + addrs[0].IPPort(), clnt, fsl}
}

func (wc *WebClnt) request(path string, vals url.Values) ([]byte, error) {
	u, err := url.Parse(wc.baseurl + path)
	if err != nil {
		return nil, err
	}
	//encode, err :=
	//if err != nil {
	//	return nil, err
	//}
	u.RawQuery = url.QueryEscape(vals.Encode())
	dbg.DPrintf(dbg.SOCIAL_NETWORK_CLNT, "about to query: %v\n", u.String())
	resp, err := wc.clnt.Get(u.String())
	if err != nil {
		dbg.DPrintf(dbg.SOCIAL_NETWORK_CLNT, "query error: %v", err.Error())
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		dbg.DFatalf("Error from status: %v, %v", err, resp)
		return nil, fmt.Errorf("%v %s", resp.StatusCode, body)
	}
	return body, nil
}

func (wc *WebClnt) Login(u, p string) (string, error) {
	vals := url.Values{}
	vals.Set("username", u)
	vals.Set("password", p)
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

func (wc *WebClnt) ComposePost(u, uid, text, ptype string, mediastr string) (string, error) {
	vals := url.Values{}
	vals.Set("username", u)
	vals.Set("userid", uid)
	vals.Set("text", text)
	vals.Set("posttype", ptype)
	vals.Set("media", mediastr)
	body, err := wc.request("/compose", vals)
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

func (wc *WebClnt) ReadTimeline(uid, count string) (map[string]interface{}, error) {
	vals := url.Values{}
	vals.Set("userid", uid)
	vals.Set("stop", count)
	body, err := wc.request("/timeline", vals)
	if err != nil {
		return nil, err
	}
	repl := make(map[string]interface{})
	err = json.Unmarshal(body, &repl)
	if err != nil {
		return nil, err
	}
	return repl, nil
}

func (wc *WebClnt) ReadHome(uid, count string) (map[string]interface{}, error) {
	vals := url.Values{}
	vals.Set("userid", uid)
	vals.Set("stop", count)
	body, err := wc.request("/home", vals)
	if err != nil {
		return nil, err
	}
	repl := make(map[string]interface{})
	err = json.Unmarshal(body, &repl)
	if err != nil {
		return nil, err
	}
	return repl, nil
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

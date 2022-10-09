package www

import (
	"io/ioutil"
	"net/http"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/fslib"
)

const (
	VIEW = BOOK + "view/"
	EDIT = BOOK + "edit/"
	SAVE = BOOK + "save/"
)

type WWWClnt struct {
	jobname  string
	srvaddrs []string
	*fslib.FsLib
}

func MakeWWWClnt(fsl *fslib.FsLib, job string) *WWWClnt {
	addrs, err := GetJobHTTPAddrs(fsl, job)
	if err != nil {
		db.DFatalf("Error wwwd job http addrs: %v", err)
	}
	return &WWWClnt{job, addrs, fsl}
}

func addrToUrl(addr string) string {
	return "http://" + addr
}

func (clnt *WWWClnt) get(path string) ([]byte, error) {
	resp, err := http.Get(addrToUrl(clnt.srvaddrs[0]) + path)
	if err != nil {
		return []byte{}, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	return body, err
}

func (clnt *WWWClnt) post(path string, vals map[string][]string) ([]byte, error) {
	resp, err := http.PostForm(addrToUrl(clnt.srvaddrs[0])+path, vals)
	if err != nil {
		return []byte{}, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	return body, err
}

func (clnt *WWWClnt) Get(addr, name string) ([]byte, error) {
	return clnt.get(STATIC + name)
}

func (clnt *WWWClnt) View(addr string) ([]byte, error) {
	return clnt.get(VIEW)
}

func (clnt *WWWClnt) Edit(addr, book string) ([]byte, error) {
	return clnt.get(EDIT + book)
}

func (clnt *WWWClnt) Save(addr string) ([]byte, error) {
	vals := map[string][]string{
		"title": []string{"Odyssey"},
	}
	return clnt.post(SAVE+"Odyssey", vals)
}

func (clnt *WWWClnt) MatMul(addr string, n int) error {
	_, err := clnt.get(MATMUL + strconv.Itoa(n))
	return err
}

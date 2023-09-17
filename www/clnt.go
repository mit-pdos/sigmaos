package www

import (
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/procclnt"
	sp "sigmaos/sigmap"
)

type WWWClnt struct {
	jobname  string
	srvaddrs sp.Taddrs
	*fslib.FsLib
}

func NewWWWClnt(fsl *fslib.FsLib, job string) *WWWClnt {
	addrs, err := GetJobHTTPAddrs(fsl, job)
	if err != nil {
		db.DFatalf("Error wwwd job http addrs: %v", err)
	}
	return &WWWClnt{job, addrs, fsl}
}

func NewWWWClntAddr(addrs sp.Taddrs) *WWWClnt {
	return &WWWClnt{"NOJOB", addrs, nil}
}

func addrToUrl(addr string) string {
	return "http://" + addr
}

func (clnt *WWWClnt) get(path string) ([]byte, error) {
	resp, err := http.Get(addrToUrl(clnt.srvaddrs[0].Addr) + path)
	if err != nil {
		return []byte{}, err
	}
	db.DPrintf(db.WWW_CLNT, "Got response: %v", resp)
	if resp.StatusCode != http.StatusOK {
		err = errors.New(resp.Status)
		return []byte{}, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	return body, err
}

func (clnt *WWWClnt) post(path string, vals map[string][]string) ([]byte, error) {
	resp, err := http.PostForm(addrToUrl(clnt.srvaddrs[0].Addr)+path, vals)
	if err != nil {
		return []byte{}, err
	}
	if resp.StatusCode != http.StatusOK {
		err = errors.New(resp.Status)
		return []byte{}, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	return body, err
}

func (clnt *WWWClnt) GetStatic(name string) ([]byte, error) {
	return clnt.get(STATIC + name)
}

func (clnt *WWWClnt) Hello() ([]byte, error) {
	return clnt.get(HELLO)
}

func (clnt *WWWClnt) MatMul(n int) error {
	_, err := clnt.get(MATMUL + strconv.Itoa(n))
	return err
}

func (clnt *WWWClnt) ConsCPULocal(n int) error {
	_, err := clnt.get(CONS_CPU_LOCAL + strconv.Itoa(n))
	return err
}

// XXX Remove eventually, repalce with Evict
func (clnt *WWWClnt) StopServer(pclnt *procclnt.ProcClnt, pid sp.Tpid) error {
	ch := make(chan error)
	go func() {
		_, err := clnt.get(EXIT)
		ch <- err
	}()

	_, err := pclnt.WaitExit(pid)
	if err != nil {
		return err
	}

	<-ch
	return nil
}

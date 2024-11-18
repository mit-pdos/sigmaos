package k8s

import (
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/rpcclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmarpcchan"
	"sigmaos/util/k8s/proto"
)

type StatScraperClnt struct {
	*sigmaclnt.SigmaClnt
	rpccs map[string]*rpcclnt.RPCClnt
}

func NewStatScraperClnt(sc *sigmaclnt.SigmaClnt) *StatScraperClnt {
	return &StatScraperClnt{
		SigmaClnt: sc,
		rpccs:     make(map[string]*rpcclnt.RPCClnt),
	}
}

func (clnt *StatScraperClnt) GetStatScrapers() []string {
	st, err := clnt.GetDir(sp.K8S_SCRAPER)
	if err != nil {
		db.DFatalf("Error GetDir %v", err)
	}
	scrapers := sp.Names(st)
	for _, s := range scrapers {
		if _, ok := clnt.rpccs[s]; !ok {
			rpcc, err := sigmarpcchan.NewSigmaRPCClnt([]*fslib.FsLib{clnt.FsLib}, filepath.Join(sp.K8S_SCRAPER, s))
			if err != nil {
				db.DFatalf("Error NewRPCClnt: %v", err)
			}
			clnt.rpccs[s] = rpcc
		}
	}
	return scrapers
}

func (clnt *StatScraperClnt) GetGuaranteedPodCPUUtil(s, qosClass string) (float64, error) {
	req := &proto.CPUUtilRequest{
		QoSClass: qosClass,
	}
	var res proto.CPUUtilResult
	err := clnt.rpccs[s].RPC("scraper.GetCPUUtil", req, &res)
	if err != nil {
		return 0.0, err
	}
	return res.Util, nil
}

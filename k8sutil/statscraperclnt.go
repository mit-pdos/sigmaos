package k8sutil

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/k8sutil/proto"
	"sigmaos/rpcclnt"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type StatScraperClnt struct {
	*sigmaclnt.SigmaClnt
	pdcs map[string]*rpcclnt.RPCClnt
}

func NewStatScraperClnt(sc *sigmaclnt.SigmaClnt) *StatScraperClnt {
	return &StatScraperClnt{
		SigmaClnt: sc,
		pdcs:      make(map[string]*rpcclnt.RPCClnt),
	}
}

func (clnt *StatScraperClnt) GetStatScrapers() []string {
	st, err := clnt.GetDir(sp.K8S_SCRAPER)
	if err != nil {
		db.DFatalf("Error GetDir %v", err)
	}
	scrapers := sp.Names(st)
	for _, s := range scrapers {
		if _, ok := clnt.pdcs[s]; !ok {
			pdc, err := rpcclnt.MkRPCClnt([]*fslib.FsLib{clnt.FsLib}, path.Join(sp.K8S_SCRAPER, s))
			if err != nil {
				db.DFatalf("Error MakeRPCClnt: %v", err)
			}
			clnt.pdcs[s] = pdc
		}
	}
	return scrapers
}

func (clnt *StatScraperClnt) GetGuaranteedPodCPUUtil(s string) (float64, error) {
	req := &proto.CPUUtilRequest{
		QoSClass: "Guaranteed",
	}
	var res proto.CPUUtilResult
	err := clnt.pdcs[s].RPC("StatScraper.GetCPUUtil", req, &res)
	if err != nil {
		return 0.0, err
	}
	return res.Util, nil
}

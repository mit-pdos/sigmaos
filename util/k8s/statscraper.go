package k8s

import (
	"fmt"

	"sigmaos/api/fs"
	"sigmaos/dcontainer/cgroup"
	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/util/k8s/proto"
)

// Kubernetes cgroups paths.
const (
	K8S_CGROUP           = "/cgroup/kubepods.slice"
	QOS_BE_CGROUP        = K8S_CGROUP + "/" + "kubepods-besteffort.slice"
	QOS_BURSTABLE_CGROUP = K8S_CGROUP + "/" + "kubepods-burstable.slice"
)

type scraper struct {
	cmon *cgroup.CgroupMonitor
}

func RunK8sStatScraper() error {
	s := newScraper()
	ssrv, err := sigmasrv.NewSigmaSrv(sp.K8S_SCRAPER, s, proc.GetProcEnv())
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}

	return ssrv.RunServer()
}

func newScraper() *scraper {
	return &scraper{
		cmon: cgroup.NewCgroupMonitor(),
	}
}

func (s *scraper) GetCPUUtil(ctx fs.CtxI, req proto.CPUUtilReq, res *proto.CPUUtilRep) error {
	var total *cgroup.CPUStat
	var be *cgroup.CPUStat
	var burst *cgroup.CPUStat
	var err error
	if be, err = s.cmon.GetCPUStats(QOS_BE_CGROUP); err != nil {
		db.DPrintf(db.ERROR, "Error BE: %v", err)
		return err
	}
	if burst, err = s.cmon.GetCPUStats(QOS_BURSTABLE_CGROUP); err != nil {
		db.DPrintf(db.ERROR, "Error burstable: %v", err)
		return err
	}
	if total, err = s.cmon.GetCPUStats(K8S_CGROUP); err != nil {
		db.DPrintf(db.ERROR, "Error total: %v", err)
		return err
	}
	db.DPrintf(db.K8S_UTIL, "Total %v BE %v Burst %v", total.Util, be.Util, burst.Util)
	// Guaranteed QoS class is total CPU utillzation, minus BE & Burstable
	// classes' utilization.
	switch req.QoSClass {
	case "BestEffort":
		res.Util = be.Util
	case "Burstable":
		res.Util = burst.Util
	case "Guaranteed":
		res.Util = total.Util - be.Util - burst.Util
	default:
		return fmt.Errorf("Error: QoS class \"%v\" unsupported", req.QoSClass)
	}
	return nil
}

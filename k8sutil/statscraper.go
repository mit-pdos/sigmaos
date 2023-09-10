package k8sutil

import (
	"fmt"

	"sigmaos/cgroup"
	"sigmaos/proc"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/k8sutil/proto"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
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
	ssrv, err := sigmasrv.MakeSigmaSrv(sp.K8S_SCRAPER, s, proc.GetProcEnv())
	if err != nil {
		db.DFatalf("Error MakeSigmaSrv: %v", err)
	}

	return ssrv.RunServer()
}

func newScraper() *scraper {
	return &scraper{
		cmon: cgroup.NewCgroupMonitor(),
	}
}

func (s *scraper) GetCPUUtil(ctx fs.CtxI, req proto.CPUUtilRequest, res *proto.CPUUtilResult) error {
	if req.QoSClass != "Guaranteed" {
		return fmt.Errorf("Error: QoS class \"%v\" unsupported", req.QoSClass)
	}
	var total *cgroup.CPUStat
	var be *cgroup.CPUStat
	var burst *cgroup.CPUStat
	var err error
	if be, err = s.cmon.GetCPUStats(QOS_BE_CGROUP); err != nil {
		db.DFatalf("Error BE: %v", err)
	}
	if burst, err = s.cmon.GetCPUStats(QOS_BURSTABLE_CGROUP); err != nil {
		db.DFatalf("Error burstable: %v", err)
	}
	if total, err = s.cmon.GetCPUStats(K8S_CGROUP); err != nil {
		db.DFatalf("Error total: %v", err)
	}
	db.DPrintf(db.K8S_UTIL, "Total %v BE %v Burst %v", total.Util, be.Util, burst.Util)
	// Guaranteed QoS class is total CPU utillzation, minus BE & Burstable
	// classes' utilization.
	res.Util = total.Util - be.Util - burst.Util
	return nil
}

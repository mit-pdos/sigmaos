package k8sutil

import (
	"fmt"

	"sigmaos/cgroup"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/k8sutil/proto"
	"sigmaos/protdevsrv"
	sp "sigmaos/sigmap"
)

// Kubernetes cgroups paths.
const (
	K8S_CGROUP           = "/cgroup/kubepods.slice"
	QOS_BE_CGROUP        = K8S_CGROUP + "/" + "kubepods.besteffort"
	QOS_BURSTABLE_CGROUP = K8S_CGROUP + "/" + "kubepods.burstable"
)

type scraper struct {
	cmon *cgroup.CgroupMonitor
}

func RunK8sStatScraper() error {
	s := newScraper()
	pds, err := protdevsrv.MakeProtDevSrv(sp.K8S_SCRAPER, s)
	if err != nil {
		db.DFatalf("Error MakeProtDevSrv: %v", err)
	}

	return pds.RunServer()
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
	if total, err = s.cmon.GetCPUStats(QOS_BURSTABLE_CGROUP); err != nil {
		db.DFatalf("Error total: %v", err)
	}
	// Guaranteed QoS class is total CPU utillzation, minus BE & Burstable
	// classes' utilization.
	res.Util = total.Util - be.Util - burst.Util
	return nil
}

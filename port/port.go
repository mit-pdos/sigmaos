package port

import (
	"fmt"
	"net"

	"github.com/docker/go-connections/nat"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type PortBinding struct {
	RealmPort sp.Tport
	HostPort  sp.Tport
}

func (pb *PortBinding) String() string {
	return fmt.Sprintf("{R %s H %s}", pb.RealmPort, pb.HostPort)
}

func (pb *PortBinding) Mark(port sp.Tport) {
	db.DPrintf(db.BOOT, "AllocPort: %v\n", port)
	pb.RealmPort = port
}

type Range struct {
	Fport sp.Tport
	Lport sp.Tport
}

func (pr *Range) String() string {
	return fmt.Sprintf("%d-%d", pr.Fport, pr.Lport)
}

type PortMap struct {
	portmap map[sp.Tport]*PortBinding
}

func NewPortMap(ports nat.PortMap, ctrports []sp.Tport) *PortMap {
	pm := &PortMap{portmap: make(map[sp.Tport]*PortBinding)}
	for _, i := range ctrports {
		p, err := nat.NewPort("tcp", i.String())
		if err != nil {
			break
		}
		for _, p := range ports[p] {
			ip := net.ParseIP(p.HostIP)
			pp, err := sp.ParsePort(p.HostPort)
			if ip.To4() != nil && err == nil {
				pm.portmap[i] = &PortBinding{HostPort: pp, RealmPort: sp.NO_PORT}
				break
			}
		}
	}
	return pm
}

func (pm *PortMap) String() string {
	s := ""
	for p, pb := range pm.portmap {
		s += fmt.Sprintf("{%v: %v} ", p, pb)
	}
	return s
}

func (pm *PortMap) GetPortBinding(port sp.Tport) (*PortBinding, error) {
	pb, ok := pm.portmap[port]
	if !ok {
		return nil, fmt.Errorf("Unknown port %s", port)
	}
	return pb, nil
}

package port

import (
	"fmt"
	"net"
	"sync"

	"github.com/docker/go-connections/nat"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

const (
	N = 20
)

// XXX type?
func NewPublicAddrs(outerIP sp.Tip, pb PortBinding, netns string, laddr *sp.Taddr) sp.Taddrs {
	addrs := make(sp.Taddrs, 2)
	addrs[0] = laddr
	addrs[1] = sp.NewTaddr(outerIP, sp.OUTER_CONTAINER_IP, pb.HostPort)
	return addrs
}

func NewPublicEndpoint(outerIP sp.Tip, pb PortBinding, net string, lmnt *sp.Tendpoint) *sp.Tendpoint {
	return sp.NewEndpoint(sp.INTERNAL_EP, NewPublicAddrs(outerIP, pb, net, lmnt.Addrs()[0]), lmnt.GetRealm())
}

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
	sync.Mutex
	portmap map[sp.Tport]*PortBinding
	fport   sp.Tport
}

func NewPortMap(ports nat.PortMap, r *Range) *PortMap {
	if r == nil {
		return nil
	}
	pm := &PortMap{fport: r.Fport, portmap: make(map[sp.Tport]*PortBinding)}
	for i := r.Fport; i < r.Lport; i++ {
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
	pm.Lock()
	defer pm.Unlock()

	s := ""
	for p, pb := range pm.portmap {
		s += fmt.Sprintf("{%v: %v} ", p, pb)
	}
	return s
}

func (pm *PortMap) AllocFirst() (*PortBinding, error) {
	return pm.AllocPortOne(pm.fport)
}

func (pm *PortMap) GetBinding(port sp.Tport) (*PortBinding, error) {
	pm.Lock()
	defer pm.Unlock()

	pb, ok := pm.portmap[port]
	if !ok {
		return nil, fmt.Errorf("Unknown port %s", port)
	}
	return pb, nil
}

func (pm *PortMap) AllocPortOne(port sp.Tport) (*PortBinding, error) {
	if pm == nil {
		return nil, fmt.Errorf("No overlay network")
	}
	pm.Lock()
	defer pm.Unlock()

	pb := pm.portmap[port]
	if pb.RealmPort == sp.NO_PORT {
		pb.Mark(port)
		return pb, nil
	}
	return nil, fmt.Errorf("Port %v already in use", port)
}

func (pm *PortMap) AllocPort() (*PortBinding, error) {
	if pm == nil {
		return nil, fmt.Errorf("No overlay network")
	}
	pm.Lock()
	defer pm.Unlock()

	for p, pb := range pm.portmap {
		if pb.RealmPort == sp.NO_PORT {
			pb.Mark(p)
			return pb, nil
		}
	}
	return nil, fmt.Errorf("Out of ports")
}

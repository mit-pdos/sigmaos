package port

import (
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/docker/go-connections/nat"

	db "sigmaos/debug"
)

type Tport int

const (
	NOPORT Tport = 0
	N            = 20
)

func (p Tport) String() string {
	return strconv.Itoa(int(p))
}

func StringToPort(s string) (Tport, error) {
	p, err := strconv.Atoi(s)
	return Tport(p), err
}

type PortBinding struct {
	RealmPort Tport
	HostPort  Tport
}

func (pb *PortBinding) String() string {
	return fmt.Sprintf("{R %s H %s}", pb.RealmPort, pb.HostPort)
}

func (pb *PortBinding) Mark(port Tport) {
	db.DPrintf(db.BOOT, "AllocPort: %v\n", port)
	pb.RealmPort = port
}

type PortRange struct {
	fport Tport
	lport Tport
}

func MakePortRange(fport, lport Tport) *PortRange {
	return &PortRange{fport, lport}
}

func (pr *PortRange) AllocRange() (Tport, Tport, error) {
	if pr.fport+N > pr.lport {
		return NOPORT, NOPORT, fmt.Errorf("Out of ports")
	}
	f := pr.fport
	l := pr.lport + N
	pr.fport = l + 1
	return f, l, nil
}

type PortMap struct {
	sync.Mutex
	portmap map[Tport]*PortBinding
	fport   Tport
}

func MakePortMap(ports nat.PortMap, fport, lport Tport) *PortMap {
	pm := &PortMap{fport: fport, portmap: make(map[Tport]*PortBinding)}
	for i := fport; i < lport; i++ {
		p, err := nat.NewPort("tcp", i.String())
		if err != nil {
			break
		}
		for _, p := range ports[p] {
			ip := net.ParseIP(p.HostIP)
			pp, err := StringToPort(p.HostPort)
			if ip.To4() != nil && err == nil {
				pm.portmap[i] = &PortBinding{HostPort: pp, RealmPort: NOPORT}
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

func (pm *PortMap) GetBinding(port Tport) (*PortBinding, error) {
	pm.Lock()
	defer pm.Unlock()

	pb, ok := pm.portmap[port]
	if !ok {
		return nil, fmt.Errorf("Unknown port %s", port)
	}
	return pb, nil
}

func (pm *PortMap) AllocPortOne(port Tport) (*PortBinding, error) {
	pm.Lock()
	defer pm.Unlock()

	pb := pm.portmap[port]
	if pb.RealmPort == NOPORT {
		pb.Mark(port)
		return pb, nil
	}
	return nil, fmt.Errorf("Port %v already in use", port)
}

func (pm *PortMap) AllocPort() (*PortBinding, error) {
	pm.Lock()
	defer pm.Unlock()

	for p, pb := range pm.portmap {
		if pb.RealmPort == NOPORT {
			pb.Mark(p)
			return pb, nil
		}
	}
	return nil, fmt.Errorf("Out of ports")
}

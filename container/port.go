package container

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
	FPORT       Tport = 1112
	LPORT       Tport = 1122
	UPROCD_PORT Tport = FPORT
)

func (p Tport) String() string {
	return strconv.Itoa(int(p))
}

type PortBinding struct {
	RealmPort string
	HostPort  string
}

type PortMap struct {
	sync.Mutex
	portmap map[string]*PortBinding
}

func makePortMap(ports nat.PortMap) *PortMap {
	pm := &PortMap{}
	pm.portmap = make(map[string]*PortBinding)
	for i := FPORT; i < LPORT; i++ {
		p, err := nat.NewPort("tcp", i.String())
		if err != nil {
			break
		}
		for _, p := range ports[p] {
			ip := net.ParseIP(p.HostIP)
			if ip.To4() != nil {
				pm.portmap[i.String()] = &PortBinding{HostPort: p.HostPort, RealmPort: ""}
				break
			}
		}
	}
	pb := pm.portmap[FPORT.String()]
	pb.Mark(FPORT.String())
	return pm
}

func (pb *PortBinding) Mark(port string) {
	db.DPrintf(db.BOOT, "AllocPort: %v\n", port)
	pb.RealmPort = port
}

func (pm *PortMap) GetPort(port string) (*PortBinding, error) {
	pm.Lock()
	defer pm.Unlock()

	pb, ok := pm.portmap[port]
	if !ok {
		return nil, fmt.Errorf("Unknown port %s", port)
	}
	return pb, nil
}

func (pm *PortMap) AllocPortOne(port string) (*PortBinding, error) {
	pm.Lock()
	defer pm.Unlock()

	pb := pm.portmap[port]
	if pb.RealmPort != "" {
		pb.Mark(port)
		return pb, nil
	}
	return nil, fmt.Errorf("Port %v already in use", port)
}

func (pm *PortMap) AllocPort() (*PortBinding, error) {
	pm.Lock()
	defer pm.Unlock()

	for p, pb := range pm.portmap {
		if pb.RealmPort == "" {
			pb.Mark(p)
			return pb, nil
		}
	}
	return nil, fmt.Errorf("Out of ports")
}

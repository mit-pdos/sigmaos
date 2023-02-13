package container

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/docker/go-connections/nat"

	"sigmaos/port"
	sp "sigmaos/sigmap"
)

func TestExpose(t *testing.T) {
	const (
		FPORT port.Tport = 100
		LPORT port.Tport = 200
	)
	ports, err := nat.NewPort("tcp", FPORT.String()+"-"+LPORT.String())
	assert.Nil(t, err)
	pms, err := nat.ParsePortSpec("0.0.0.0:" + FPORT.String() + "-" + LPORT.String() + ":8112-8113")
	assert.Nil(t, err)
	pmap := nat.PortMap{}
	pmap[ports] = []nat.PortBinding{}
	log.Printf("ports %v pms  %v\n", ports, pms)
}

func TestRearrange(t *testing.T) {
	addrs := sp.Taddrs{"10.0.1.55:1113", "192.168.2.114:1113"}
	raddrs := rearrange(addrs, "192.168.2.114")
	log.Printf("addrs %v %v -> %v\n", addrs, "192.168.2.114", raddrs)

	addrs = sp.Taddrs{"192.168.2.114:1113", "10.0.1.55:1113"}
	raddrs = rearrange(addrs, "10.0.1.53")
	log.Printf("addrs %v %v -> %v\n", addrs, "10.0.1.53", raddrs)

	addrs = sp.Taddrs{"10.0.7.63:1122", "192.168.2.114:39395"}
	raddrs = rearrange(addrs, "10.0.5.61")
	log.Printf("addrs %v %v -> %v\n", addrs, "10.0.5.61", raddrs)
}

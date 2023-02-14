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
	addr0 := sp.MkTaddrRealm("10.0.1.55:1113", sp.Trealm("realm1"))
	addr1 := sp.MkTaddrRealm("10.0.7.53:1113", sp.Trealm("realm2"))
	addr2 := sp.MkTaddrRealm("192.168.2.114:1113", sp.ROOTREALM)

	addrs := sp.Taddrs{addr0, addr2}
	raddrs := Rearrange(sp.ROOTREALM, addrs)
	log.Printf("addrs %v -> %v\n", addrs, raddrs)

	addrs = sp.Taddrs{addr2, addr0}
	raddrs = Rearrange(sp.Trealm("realm1"), addrs)
	log.Printf("addrs %v -> %v\n", addrs, raddrs)

	addrs = sp.Taddrs{addr1, addr2}
	raddrs = Rearrange(sp.Trealm("realm1"), addrs)
	log.Printf("addrs %v -> %v\n", addrs, raddrs)
}

package container_test

import (
	"fmt"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/docker/go-connections/nat"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/mem"
	"sigmaos/port"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
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
	addr0 := sp.MkTaddrRealm("10.0.1.55:1113", "realm1")
	addr1 := sp.MkTaddrRealm("10.0.7.53:1113", "realm2")
	addr2 := sp.MkTaddrRealm("192.168.2.114:1113", string(sp.ROOTREALM))

	addrs := sp.Taddrs{addr0, addr2}
	raddrs := container.Rearrange(sp.ROOTREALM.String(), addrs)
	log.Printf("addrs %v -> %v\n", addrs, raddrs)

	addrs = sp.Taddrs{addr2, addr0}
	raddrs = container.Rearrange("realm1", addrs)
	log.Printf("addrs %v -> %v\n", addrs, raddrs)

	addrs = sp.Taddrs{addr1, addr2}
	raddrs = container.Rearrange("realm1", addrs)
	log.Printf("addrs %v -> %v\n", addrs, raddrs)
}

func runMemHog(ts *test.Tstate, c chan error, id, delay, mem string) {
	p := proc.MakeProc("memhog", []string{id, delay, mem})
	if id == "LC" {
		p.SetNcore(2)
	}
	err := ts.Spawn(p)
	assert.Nil(ts.T, err, "Error spawn: %v", err)
	status, err := ts.WaitExit(p.GetPid())
	if err != nil {
		c <- err
		return
	}
	if !status.IsStatusOK() {
		c <- status.Error()
		return
	}
	c <- nil
}

func TestLCAlone(t *testing.T) {
	ts := test.MakeTstateAll(t)

	mem := mem.GetTotalMem()
	lcC := make(chan error)
	go runMemHog(ts, lcC, "LC", "2s", fmt.Sprintf("%dMB", mem/2))
	r1 := <-lcC
	assert.Nil(t, r1)
	ts.Shutdown()
}

func TestReapBE(t *testing.T) {
	ts := test.MakeTstateAll(t)

	mem := mem.GetTotalMem()
	beC := make(chan error)
	lcC := make(chan error)
	go runMemHog(ts, lcC, "LC", "2s", fmt.Sprintf("%dMB", mem/2))
	go runMemHog(ts, beC, "BE", "5s", fmt.Sprintf("%dMB", (mem*3)/4))
	r := <-beC
	db.DPrintf(db.TEST, "beLC %v\n", r)
	r1 := <-lcC
	assert.Nil(t, r1)
	ts.Shutdown()
}

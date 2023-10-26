package netsigma

import (
	"log"
	"testing"

	sp "sigmaos/sigmap"
)

func TestRearrange(t *testing.T) {
	addr0 := sp.NewTaddrRealm("10.0.1.55:1113", "realm1")
	addr1 := sp.NewTaddrRealm("10.0.7.53:1113", "realm2")
	addr2 := sp.NewTaddrRealm("192.168.2.114:1113", string(sp.ROOTREALM))

	addrs := sp.Taddrs{addr0, addr2}
	raddrs := Rearrange(sp.ROOTREALM.String(), addrs)
	log.Printf("addrs %v -> %v\n", addrs, raddrs)

	addrs = sp.Taddrs{addr2, addr0}
	raddrs = Rearrange("realm1", addrs)
	log.Printf("addrs %v -> %v\n", addrs, raddrs)

	addrs = sp.Taddrs{addr1, addr2}
	raddrs = Rearrange("realm1", addrs)
	log.Printf("addrs %v -> %v\n", addrs, raddrs)
}

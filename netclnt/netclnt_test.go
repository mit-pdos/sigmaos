package netclnt

import (
	"log"

	"testing"

	sp "sigmaos/sigmap"
)

func TestRearrange(t *testing.T) {
	addrs := sp.Taddrs{"10.0.1.55:1113", "192.168.2.114:1113"}
	addrs = rearrange(addrs, "192.168.2.114")
	log.Printf("addrs %v\n", addrs)
}

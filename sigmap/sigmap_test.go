package sigmap_test

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"

	sp "sigmaos/sigmap"
)

func TestCompile(t *testing.T) {
}

func TestString(t *testing.T) {
	qt := sp.Qtype(sp.QTSYMLINK | sp.QTTMP)
	assert.Equal(t, qt.String(), "ts")

	p := sp.Tperm(0x60001ff)
	assert.Equal(t, "qt ts qp ff", p.String())
}

func TestNamedAddrs(t *testing.T) {
	addrs := make(sp.Taddrs, 2)
	addrs[0] = sp.NewTaddrRealm(sp.Thost("10.x.x.x"), 1111, "testrealm")
	addrs[1] = sp.NewTaddrRealm(sp.Thost("192.y.y.y"), 1111, "rootrealm")
	s, err := addrs.Taddrs2String()
	assert.Nil(t, err)
	as, err := sp.String2Taddrs(s)
	assert.Nil(t, err)
	log.Printf("s %v -> %v %v\n", s, as[0], as[1])
}

package sigmap

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompile(t *testing.T) {
}

func TestString(t *testing.T) {
	qt := Qtype(QTSYMLINK | QTTMP)
	assert.Equal(t, qt.String(), "ts")

	p := Tperm(0x60001ff)
	assert.Equal(t, "qt ts qp ff", p.String())
}

func TestNamedAddrs(t *testing.T) {
	addrs := make(Taddrs, 2)
	addrs[0] = NewTaddrRealm("10.x.x.x:1111", "testrealm")
	addrs[1] = NewTaddrRealm("192.y.y.y:1111", "rootrealm")
	s, err := addrs.Taddrs2String()
	assert.Nil(t, err)
	as, err := String2Taddrs(s)
	assert.Nil(t, err)
	log.Printf("s %v -> %v %v\n", s, as[0], as[1])
}

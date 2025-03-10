package sigmap_test

import (
	"log"
	"strings"
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
	assert.Equal(t, "{qt: ts qp: ff}", p.String())
}

func TestNamedAddrs(t *testing.T) {
	addrs := make(sp.Taddrs, 2)
	addrs[0] = sp.NewTaddr(sp.Tip("10.x.x.x"), 1111)
	addrs[1] = sp.NewTaddr(sp.Tip("192.y.y.y"), 1111)
	s, err := addrs.Taddrs2String()
	assert.Nil(t, err)
	as, err := sp.String2Taddrs(s)
	assert.Nil(t, err)
	log.Printf("s %v -> %v %v\n", s, as[0], as[1])
}

func TestIsSPProxydKernel(t *testing.T) {
	sckid := sp.SPProxydKernel("sigma-1c80")
	assert.True(t, strings.HasPrefix(sckid, sp.SPPROXYDKERNEL))
}

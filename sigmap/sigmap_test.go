package sigmap_test

import (
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestCompile(t *testing.T) {
	assert.NotNil(t, test.User)
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

func TestSubstLocal(t *testing.T) {
	const kid = "sigma-1c80"

	// Substituting a concrete server, and ~any, for ~local.
	pn, ok := sp.SubstLocal(sp.UX+sp.LOCAL+"/mr-intermediate", kid)
	assert.True(t, ok)
	assert.Equal(t, sp.UX+kid+"/mr-intermediate", pn)

	pn, ok = sp.SubstLocal(sp.S3+sp.LOCAL+"/9ps3/mr-out", sp.ANY)
	assert.True(t, ok)
	assert.Equal(t, sp.S3+sp.ANY+"/9ps3/mr-out", pn)

	// The bare union pathname, with nothing below it.
	pn, ok = sp.SubstLocal(sp.UX+sp.LOCAL, kid)
	assert.True(t, ok)
	assert.Equal(t, sp.UX+kid, pn)

	// Pathnames with no ~local component come back unchanged: another
	// server, ~any (which is not ~local), and a file whose name merely
	// starts with "~local".
	for _, in := range []string{
		sp.UX + "sigma-9999/mr-intermediate",
		sp.UX + sp.ANY + "/mr-intermediate",
		sp.UX + sp.LOCAL + "dir/mr-intermediate",
		"name/ux",
	} {
		pn, ok = sp.SubstLocal(in, kid)
		assert.False(t, ok, "%v", in)
		assert.Equal(t, in, pn, "%v", in)
	}
}

package coord

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/mr"
	sp "sigmaos/sigmap"
)

// rewriteBin drives the coordinator's own rewrite, so that what is tested here is
// what mapperProc runs rather than a copy of it that can drift.
func rewriteBin(bin mr.Bin, intOutdir, kid string, intOutS3 bool) (mr.Bin, string) {
	c := &Coord{dedicatedUx: []string{kid}, intOutdir: intOutdir, intOutS3: intOutS3}
	return c.rewriteForDedicatedUx(bin)
}

// With no machines dedicated to hosting the job's data, nothing is rewritten and
// every path keeps ~local — each mapper reads from and writes to its own node,
// which is what an ordinary run does.
func TestUxKernelForNoneDedicated(t *testing.T) {
	c := &Coord{}
	kid, ok := c.uxKernelFor()
	assert.False(t, ok)
	assert.Equal(t, "", kid)
}

// Mappers are spread over the dedicated machines rather than all reading from the
// first one.
func TestUxKernelForRoundRobin(t *testing.T) {
	c := &Coord{dedicatedUx: []string{"kid-a", "kid-b", "kid-c"}}
	got := make([]string, 0, 7)
	for i := 0; i < 7; i++ {
		kid, ok := c.uxKernelFor()
		assert.True(t, ok)
		got = append(got, kid)
	}
	assert.Equal(t, []string{"kid-a", "kid-b", "kid-c", "kid-a", "kid-b", "kid-c", "kid-a"}, got)
}

// Every split and the intermediate directory name the same dedicated server, so a
// mapper reads from and writes to one machine.
func TestRewriteBinUxInput(t *testing.T) {
	const kid = "sigma-42"
	bin := mr.Bin{
		{File: sp.UX + sp.LOCAL + "/input-data/wiki-128M/f0", Offset: 0, Length: 100},
		{File: sp.UX + sp.LOCAL + "/input-data/wiki-128M/f1", Offset: 100, Length: 100},
	}
	bin, intOut := rewriteBin(bin, sp.UX+sp.LOCAL+"/mr-int", kid, false)

	assert.Equal(t, sp.UX+kid+"/input-data/wiki-128M/f0", bin[0].File)
	assert.Equal(t, sp.UX+kid+"/input-data/wiki-128M/f1", bin[1].File)
	assert.Equal(t, sp.UX+kid+"/mr-int", intOut)
	// Only the server changes, not which bytes of the input this mapper owns.
	assert.Equal(t, sp.Toffset(100), bin[1].Offset)
	assert.Equal(t, sp.Tlength(100), bin[1].Length)
}

// An S3 path has a ~local of its own, naming the local S3 proxy. Replacing it with
// a UX server's kernel ID would name a proxy that doesn't exist, so an S3 input or
// an S3 intermediate directory is left exactly as it was — a dedicated-UX run of a
// job that reads from S3 changes nothing about how it reads.
func TestRewriteBinLeavesS3Alone(t *testing.T) {
	const kid = "sigma-42"
	const s3in = "name/s3/" + sp.LOCAL + "/9ps3/wiki-2G/f0"
	const s3int = "name/s3/" + sp.LOCAL + "/9ps3/mr-int"

	bin, intOut := rewriteBin(mr.Bin{{File: s3in}}, s3int, kid, true)
	assert.Equal(t, s3in, bin[0].File, "S3 input must not be rewritten to a UX server")
	assert.Equal(t, s3int, intOut, "S3 intermediate output must not be rewritten")
}

// A bin whose splits already name a concrete UX server has no ~local to replace,
// so it passes through unchanged rather than being repointed.
func TestRewriteBinLeavesConcreteUxAlone(t *testing.T) {
	const pn = sp.UX + "sigma-other/input-data/wiki-128M/f0"
	bin, _ := rewriteBin(mr.Bin{{File: pn}}, sp.UX+sp.LOCAL+"/mr-int", "sigma-42", false)
	assert.Equal(t, pn, bin[0].File)
}

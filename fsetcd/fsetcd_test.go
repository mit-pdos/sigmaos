package fsetcd_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/fsetcd"
	"sigmaos/path"
	sp "sigmaos/sigmap"
)

func TestDump(t *testing.T) {
	ec, err := fsetcd.MkEtcdClnt(sp.ROOTREALM)
	assert.Nil(t, err)
	nd, err := ec.ReadDir(fsetcd.ROOT)
	assert.Nil(t, err)
	err = ec.Dump(0, nd, path.Path{}, fsetcd.ROOT)
	assert.Nil(t, err)
}

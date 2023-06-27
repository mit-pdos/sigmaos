package fsetcd_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/fsetcd"
	"sigmaos/path"
	sp "sigmaos/sigmap"
)

func TestDump(t *testing.T) {
	fs, err := fsetcd.MkFsEtcd(sp.ROOTREALM)
	assert.Nil(t, err)
	nd, err := fs.ReadDir(fsetcd.ROOT)
	assert.Nil(t, err)
	err = fs.Dump(0, nd, path.Path{}, fsetcd.ROOT)
	assert.Nil(t, err)
}

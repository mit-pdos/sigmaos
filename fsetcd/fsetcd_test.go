package fsetcd_test

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/proc"
	"sigmaos/fsetcd"
	"sigmaos/path"
	sp "sigmaos/sigmap"
)

var realm string

func init() {
	flag.StringVar(&realm, "realm", string(sp.ROOTREALM), "realm")
}

func TestDump(t *testing.T) {
	scfg := proc.NewTestProcEnv(sp.Trealm(realm), etcdIP, "", "")
	fs, err := fsetcd.MkFsEtcd(scfg)
	assert.Nil(t, err)
	nd, err := fs.ReadDir(fsetcd.ROOT)
	assert.Nil(t, err)
	err = fs.Dump(0, nd, path.Path{}, fsetcd.ROOT)
	assert.Nil(t, err)
}

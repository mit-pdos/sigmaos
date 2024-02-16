package fsetcd_test

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/fsetcd"
	"sigmaos/path"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

var realm string

func init() {
	flag.StringVar(&realm, "realm", string(sp.ROOTREALM), "realm")
}

func TestDump(t *testing.T) {
	pcfg := proc.NewTestProcEnv(sp.Trealm(realm), nil, sp.Tip(test.EtcdIP), sp.NO_IP, sp.NO_IP, "", false, false)
	fs, err := fsetcd.NewFsEtcd(pcfg.GetRealm(), pcfg.GetEtcdIP())
	assert.Nil(t, err, "Err %v", err)
	nd, err := fs.ReadDir(fsetcd.ROOT)
	assert.Nil(t, err, "Err %v", err)
	err = fs.Dump(0, nd, path.Path{}, fsetcd.ROOT)
	assert.Nil(t, err, "Err %v", err)
}

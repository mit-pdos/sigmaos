package fsetcd_test

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/fsetcd"
	"sigmaos/netproxyclnt"
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
	lip := sp.Tip("127.0.0.1")
	_, _, amgr, err := test.NewAuthMgr()
	assert.Nil(t, err)
	secrets := map[string]*proc.ProcSecretProto{}
	etcdMnt, err := fsetcd.NewFsEtcdEndpoint(amgr, sp.Tip(test.EtcdIP))
	pe := proc.NewTestProcEnv(sp.ROOTREALM, secrets, etcdMnt, lip, lip, "", false, false, false, false)
	npc := netproxyclnt.NewNetProxyClnt(pe, nil)
	fs, err := fsetcd.NewFsEtcd(npc.Dial, pe.GetEtcdEndpoints(), pe.GetRealm())
	assert.Nil(t, err, "Err %v", err)
	nd, err := fs.ReadDir(fsetcd.ROOT)
	assert.Nil(t, err, "Err %v", err)
	err = fs.Dump(0, nd, path.Path{}, fsetcd.ROOT)
	assert.Nil(t, err, "Err %v", err)
}

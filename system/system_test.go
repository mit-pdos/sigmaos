package system_test

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/fslib"
	"sigmaos/named"
	"sigmaos/realm"
	"sigmaos/system"
)

type Tstate struct {
	*system.System
	T *testing.T
}

func mkTstate(t *testing.T) *Tstate {
	sys, err := system.Boot("testrealm", "../bootkernelclnt")
	assert.Nil(t, err)
	return &Tstate{sys, t}
}

func (ts *Tstate) Shutdown() {
	err := ts.System.Shutdown()
	assert.Nil(ts.T, err)
}

func TestStartStop(t *testing.T) {
	ts := mkTstate(t)
	pn := path.Join(realm.REALM_NAMEDS, "testrealm")
	sts, err := ts.Root.GetDir(pn + "/")
	assert.Nil(t, err)
	assert.True(t, fslib.Present(sts, named.InitRootDir))
	ts.Shutdown()
}

package test

import (
	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/util/crash"
)

func (ts *Tstate) CrashServer(e0, e1 crash.Tevent, srv string) {
	db.DPrintf(db.ALWAYS, "Crash %v srv %v", e0.Path, srv)
	err := crash.SignalFailer(ts.FsLib, e0.Path)
	if !assert.Nil(ts.T, err, "Err SignalFailer: %v", err) {
		db.DPrintf(db.TEST, "SignalFailer %v err %v", e0.Path, err)
	}
	em := crash.NewTeventMapOne(e1)
	s, err := em.Events2String()
	assert.Nil(ts.T, err)
	switch srv {
	case sp.MSCHEDREL, sp.PROCDREL, sp.UXREL:
		// a crashed msched, procd, and ux causes several kernel services
		// to exit, so start a new node.
		err = ts.BootNode(1)
	default:
		err = ts.BootEnv(srv, []string{"SIGMAFAIL=" + s})
	}
	assert.Nil(ts.T, err, "Error Boot: %v", err)
	db.DPrintf(db.ALWAYS, "Booted %v %v", e1.Path, em)
}

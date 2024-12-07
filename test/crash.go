package test

import (
	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/util/crash"
)

func (ts *Tstate) CrashServer1(e0, e1 crash.Tevent) {
	err := crash.SetSigmaFail(crash.NewTeventMapOne(e1))
	assert.Nil(ts.T, err)
	db.DPrintf(db.ALWAYS, "Booting %v node Before crashing a %v.", e1.Path, e0.Path)
	err = ts.BootNode(1)
	db.DPrintf(db.ALWAYS, "Done booting a node before crashing a %v.", e0.Path)
	if !assert.Nil(ts.T, err) {
		db.DPrintf(db.ERROR, "Error BootNode %v", e1.Path)
	}
	err = crash.SignalFailer(ts.FsLib, e0.Path)
	if !assert.Nil(ts.T, err) {
		db.DPrintf(db.ERROR, "Error non-nil kill %v: %v", e0.Path, err)
	}
	db.DPrintf(db.ALWAYS, "Done crash one %v", e0.Path)
}

func (ts *Tstate) CrashUx(e0, e1 crash.Tevent) {
	db.DPrintf(db.ALWAYS, "Crash %v", e0.Path)
	err := crash.SignalFailer(ts.FsLib, e0.Path)
	if !assert.Nil(ts.T, err) {
		db.DPrintf(db.ERROR, "Error non-nil kill %v: %v", e0.Path, err)
	}
	em := crash.NewTeventMapOne(e1)
	s, err := em.Events2String()
	assert.Nil(ts.T, err)
	err = ts.BootEnv(sp.UXREL, []string{"SIGMAFAIL=" + s})
	assert.Nil(ts.T, err)
	db.DPrintf(db.ALWAYS, "Booted %v %v", e1.Path, em)
}

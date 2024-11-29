package imgresize_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"sigmaos/apps/imgresize"
	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
	rd "sigmaos/util/rand"
)

type TstateRPC struct {
	job string
	*test.Tstate
	srvProc *proc.Proc
	rpcc    *imgresize.ImgResizeRPCClnt
}

func newTstateRPC(t *test.Tstate) (*TstateRPC, error) {
	ts := &TstateRPC{}
	ts.Tstate = t
	ts.job = rd.String(4)
	ts.cleanup()

	err := ts.MkDir(sp.IMG, 0777)
	if !assert.Nil(ts.T, err) {
		return nil, err
	}
	p, err := imgresize.StartImgRPCd(ts.SigmaClnt, ts.job, IMG_RESIZE_MCPU, IMG_RESIZE_MEM, 1, 0)
	if !assert.Nil(ts.T, err) {
		return nil, err
	}
	db.DPrintf(db.TEST, "Started imgd RPC server")
	ts.srvProc = p
	rpcc, err := imgresize.NewImgResizeRPCClnt(ts.SigmaClnt.FsLib, ts.job)
	if !assert.Nil(ts.T, err) {
		return nil, err
	}
	ts.rpcc = rpcc
	return ts, nil
}

func (ts *TstateRPC) cleanup() {
	ts.RmDir(sp.IMG)
	imgresize.Cleanup(ts.FsLib, filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save"))
}

func (ts *TstateRPC) shutdown() {
	err := ts.Evict(ts.srvProc.GetPid())
	assert.Nil(ts.T, err)
	status, err := ts.WaitExit(ts.srvProc.GetPid())
	if assert.Nil(ts.T, err) {
		assert.True(ts.T, status.IsStatusEvicted(), "Wrong status: %v", status)
	}
	ts.Shutdown()
}

func TestImgdRPC(t *testing.T) {
	t1, err1 := test.NewTstateAll(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ts, err1 := newTstateRPC(t1)
	if !assert.Nil(t, err1, "Error New Tstate2: %v", err1) {
		t1.Shutdown()
		return
	}

	err := ts.BootNode(1)
	assert.Nil(t, err, "BootProcd 1")

	err = ts.BootNode(1)
	assert.Nil(t, err, "BootProcd 2")

	in := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/6.jpg")
	err = ts.rpcc.Resize("resize-rpc-test", in)
	assert.Nil(ts.T, err)
	ts.shutdown()
}

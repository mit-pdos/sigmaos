package imgresize_test

import (
	"os"
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
	job     string
	mrts    *test.MultiRealmTstate
	srvProc *proc.Proc
	rpcc    *imgresize.ImgResizeRPCClnt
}

func newTstateRPC(mrts *test.MultiRealmTstate) (*TstateRPC, error) {
	ts := &TstateRPC{}
	ts.mrts = mrts
	ts.job = rd.String(4)
	ts.cleanup()

	err := ts.mrts.GetRealm(test.REALM1).MkDir(sp.IMG, 0777)
	if !assert.Nil(ts.mrts.T, err) {
		return nil, err
	}
	p, err := imgresize.StartImgRPCd(ts.mrts.GetRealm(test.REALM1).SigmaClnt, ts.job, IMG_RESIZE_MCPU, IMG_RESIZE_MEM, 1, 0)
	if !assert.Nil(ts.mrts.T, err) {
		return nil, err
	}
	db.DPrintf(db.TEST, "Started imgd RPC server")
	ts.srvProc = p
	rpcc, err := imgresize.NewImgResizeRPCClnt(ts.mrts.GetRealm(test.REALM1).SigmaClnt.FsLib, ts.job)
	if !assert.Nil(ts.mrts.T, err) {
		return nil, err
	}
	ts.rpcc = rpcc
	return ts, nil
}

func (ts *TstateRPC) cleanup() {
	ts.mrts.GetRealm(test.REALM1).RmDir(sp.IMG)
	imgresize.Cleanup(ts.mrts.GetRealm(test.REALM1).FsLib, filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save"))
}

func (ts *TstateRPC) shutdown() {
	err := ts.mrts.GetRealm(test.REALM1).Evict(ts.srvProc.GetPid())
	assert.Nil(ts.mrts.T, err)
	status, err := ts.mrts.GetRealm(test.REALM1).WaitExit(ts.srvProc.GetPid())
	if assert.Nil(ts.mrts.T, err) {
		assert.True(ts.mrts.T, status.IsStatusEvicted(), "Wrong status: %v", status)
	}
}

func TestImgdRPCSigmaOS(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	ts, err1 := newTstateRPC(mrts)
	if !assert.Nil(t, err1, "Error New Tstate2: %v", err1) {
		return
	}

	err := ts.mrts.GetRealm(test.REALM1).BootNode(1)
	assert.Nil(t, err, "BootProcd 1")

	err = ts.mrts.GetRealm(test.REALM1).BootNode(1)
	assert.Nil(t, err, "BootProcd 2")

	in := filepath.Join(sp.S3, sp.LOCAL, "9ps3/img-save/6.jpg")
	err = ts.rpcc.Resize("resize-rpc-test", in)
	assert.Nil(ts.mrts.T, err)
	ts.shutdown()
}

func TestImgdRPCOS(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	ts, err1 := newTstateRPC(mrts)
	if !assert.Nil(t, err1, "Error New Tstate2: %v", err1) {
		return
	}

	// Test expects 6.jpg to already be ibn /tmp/sigmaos-perf
	if _, err := os.Stat("/tmp/sigmaos-perf/6.jpg"); err == nil {
		in := filepath.Join("/tmp/sigmaos-perf/6.jpg")
		err := ts.rpcc.Resize("resize-rpc-test", in)
		assert.Nil(ts.mrts.T, err)
	}

	ts.shutdown()
}

func TestImgdDockerOS(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	// Test expects 6.jpg to already be ibn /tmp/sigmaos-perf
	if _, err := os.Stat("/tmp/sigmaos-perf/6.jpg"); err == nil {
		in := filepath.Join("/tmp/sigmaos-perf/6.jpg")
		if err := imgresize.RunImgresizeProcViaDocker(mrts.GetRealm(test.REALM1).ProcEnv(), in, 1); !assert.Nil(t, err, "Err run via docker: %v", err) {
			return
		}
	}
}

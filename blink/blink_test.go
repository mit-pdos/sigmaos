package blink_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	imgrec_py "sigmaos/apps/imgrec/py"
	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	imgBucket   = "9ps3"
	imgKey      = "img-save/8.jpg"
	modelBucket = "9ps3"
	modelKey    = "mobilenetv2-12.onnx"
	kid         = "~local"
)

// startBlinkd starts blinkd and registers a t.Cleanup to kill it when the test ends.
// Returns false if startup failed.
func startBlinkd(t *testing.T) bool {
	_, testFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(testFile), "..")
	blinkdBin := filepath.Join(repoRoot, "bin", "kernel", "blinkd")
	logFile, err := os.Create("/tmp/blinkd.out")
	if !assert.Nil(t, err, "blinkd log create: %v", err) {
		return false
	}
	cmd := exec.Command(blinkdBin, "test")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		assert.Nil(t, err, "blinkd Start: %v", err)
		logFile.Close()
		return false
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		logFile.Close()
	})
	// Give blinkd time to finish Caladan setup and start listening.
	time.Sleep(2 * time.Second)
	return true
}

func TestImgrecBlink(t *testing.T) {
	if !startBlinkd(t) {
		return
	}

	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)

	conf := imgrec_py.NewImgrecPyJobConfig(imgBucket, imgKey, modelBucket, modelKey, kid, false, false, 0, 0)
	job, err := imgrec_py.NewImgrecBlinkJob(conf, rts.SigmaClnt)
	if !assert.Nil(t, err, "NewImgrecBlinkJob: %v", err) {
		return
	}

	msg, err := job.Run(sp.NOT_SET)
	assert.Nil(t, err, "Run: %v", err)
	db.DPrintf(db.TEST, "imgrec blink pred: %v", msg)
}

func TestImgrecBlinkCoSandbox(t *testing.T) {
	if !startBlinkd(t) {
		return
	}

	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)

	conf := imgrec_py.NewImgrecPyJobConfig(imgBucket, imgKey, modelBucket, modelKey, kid, true, false, 0, 0)
	job, err := imgrec_py.NewImgrecBlinkJob(conf, rts.SigmaClnt)
	if !assert.Nil(t, err, "NewImgrecBlinkJob: %v", err) {
		return
	}

	msg, err := job.Run(sp.NOT_SET)
	assert.Nil(t, err, "Run: %v", err)
	db.DPrintf(db.TEST, "imgrec blink cosandbox pred: %v", msg)
}

func TestImgrecBlinkShmemCoSandbox(t *testing.T) {
	if !startBlinkd(t) {
		return
	}

	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)

	conf := imgrec_py.NewImgrecPyJobConfig(imgBucket, imgKey, modelBucket, modelKey, kid, true, false, proc.Tmem(256), 0)
	job, err := imgrec_py.NewImgrecBlinkJob(conf, rts.SigmaClnt)
	if !assert.Nil(t, err, "NewImgrecBlinkJob: %v", err) {
		return
	}

	msg, err := job.Run(sp.NOT_SET)
	assert.Nil(t, err, "Run: %v", err)
	db.DPrintf(db.TEST, "imgrec blink shmem pred: %v", msg)
}

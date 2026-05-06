package blink_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	imgrec_py "sigmaos/apps/imgrec/py"
	imgrectestutil "sigmaos/apps/imgrec/testutil"
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	imgBucket   = "9ps3"
	imgKey      = "img-save/8.jpg"
	modelBucket = "9ps3"
	modelKey    = "mobilenetv2-12.onnx"
	kid = "~local"
)

func TestImgrecBlink(t *testing.T) {
	_, testFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(testFile), "..")
	blinkdBin := filepath.Join(repoRoot, "bin", "kernel", "blinkd")
	cmd := exec.Command(blinkdBin, "test")
	if err := cmd.Start(); err != nil {
		assert.Nil(t, err, "blinkd Start: %v", err)
		return
	}
	defer cmd.Process.Kill()

	// Give blinkd time to finish Caladan setup and start listening.
	time.Sleep(2 * time.Second)

	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)
	ref := imgrectestutil.GetReferenceOutput(t, rts.FsLib, imgBucket, imgKey, modelBucket, modelKey, kid)

	conf := imgrec_py.NewImgrecPyJobConfig(imgBucket, imgKey, modelBucket, modelKey, kid, false, false, 0, 0)
	job := imgrec_py.NewImgrecBlinkJob(conf, rts.SigmaClnt)

	msg, err := job.Run(sp.NOT_SET)
	assert.Nil(t, err, "Run: %v", err)
	db.DPrintf(db.TEST, "imgrec blink pred: %v", msg)
	imgrectestutil.AssertMatchesReference(t, msg, ref)
}

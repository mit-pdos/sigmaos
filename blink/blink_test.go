package blink_test

import (
	"flag"
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

	uxImgPath   = "9.jpg"
	uxModelPath = "mobilenetv2-12.onnx"
)

var modelLocalPath = flag.String("model-local-path", "/model-data/mobilenetv2-12.onnx", "local filesystem path for model weights; empty string fetches from S3")
var nfsAddr = flag.String("nfs", "", "NFS server IP address; if set, passed to blinkd for the chroot mount script")
var useUX = flag.Bool("use-ux", false, "fetch image and model from UX filesystem instead of S3")

// inputBucketKey returns the imgBucket, imgKey, modelBucket, modelKey to use
// based on whether --use-ux is set.
func inputBucketKey() (string, string, string, string) {
	if *useUX {
		return "ux", uxImgPath, "ux", uxModelPath
	}
	return imgBucket, imgKey, modelBucket, modelKey
}

// copyInputToUX copies the input image from S3 to UX. Returns false if the copy failed.
func copyInputToUX(t *testing.T, sc interface {
	CopyFile(sp.Tsigmapath, sp.Tsigmapath) error
}) bool {
	src := sp.Tsigmapath(filepath.Join(sp.S3, kid, "9ps3", "img-save", uxImgPath))
	dst := sp.Tsigmapath(filepath.Join(sp.UX, kid, uxImgPath))
	return assert.Nil(t, sc.CopyFile(src, dst), "CopyFile %v -> %v", src, dst)
}

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
	args := []string{}
	if *nfsAddr != "" {
		args = append(args, "--nfs", *nfsAddr)
	}
	args = append(args, "test")
	cmd := exec.Command(blinkdBin, args...)
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

func TestImgrecBlinkSnapshot(t *testing.T) {
	if !startBlinkd(t) {
		return
	}

	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)

	if *useUX && !copyInputToUX(t, rts.SigmaClnt) {
		return
	}

	ib, ik, mb, mk := inputBucketKey()
	conf := imgrec_py.NewImgrecPyJobConfig(ib, ik, mb, mk, kid, false, false, 0, 0, *modelLocalPath)
	job, err := imgrec_py.NewImgrecBlinkJob(conf, rts.SigmaClnt)
	if !assert.Nil(t, err, "NewImgrecBlinkJob: %v", err) {
		return
	}

	msg, err := job.Run(sp.NOT_SET)
	assert.Nil(t, err, "Run: %v", err)
	db.DPrintf(db.TEST, "imgrec blink pred: %v", msg)
}

func TestImgrecBlinkShmemBench(t *testing.T) {
	if !startBlinkd(t) {
		return
	}

	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)

	if *useUX && !copyInputToUX(t, rts.SigmaClnt) {
		return
	}

	ib, ik, mb, mk := inputBucketKey()
	conf := imgrec_py.NewImgrecPyJobConfig(ib, ik, mb, mk, kid, false, false, proc.Tmem(256), 0, *modelLocalPath)
	job, err := imgrec_py.NewImgrecBlinkJob(conf, rts.SigmaClnt)
	if !assert.Nil(t, err, "NewImgrecBlinkJob: %v", err) {
		return
	}

	msg, err := job.Run(sp.NOT_SET)
	assert.Nil(t, err, "Run: %v", err)
	db.DPrintf(db.TEST, "imgrec blink cosandbox pred: %v", msg)
}

func TestImgrecBlinkCoSandboxBench(t *testing.T) {
	if !startBlinkd(t) {
		return
	}

	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)

	if *useUX && !copyInputToUX(t, rts.SigmaClnt) {
		return
	}

	ib, ik, mb, mk := inputBucketKey()
	conf := imgrec_py.NewImgrecPyJobConfig(ib, ik, mb, mk, kid, true, false, proc.Tmem(256), 0, *modelLocalPath)
	job, err := imgrec_py.NewImgrecBlinkJob(conf, rts.SigmaClnt)
	if !assert.Nil(t, err, "NewImgrecBlinkJob: %v", err) {
		return
	}

	msg, err := job.Run(sp.NOT_SET)
	assert.Nil(t, err, "Run: %v", err)
	db.DPrintf(db.TEST, "imgrec blink shmem pred: %v", msg)
}

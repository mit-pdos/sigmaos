package imgrec_py_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	imgrec_py "sigmaos/apps/imgrec/py"
	imgrectestutil "sigmaos/apps/imgrec/testutil"
	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	IMG_BUCKET   = "9ps3"
	IMG_KEY      = "img-save/8.jpg"
	MODEL_BUCKET = "9ps3"
	MODEL_KEY    = "mobilenetv2-12.onnx"
	KID          = "~local"
)

func TestImgrecPy(t *testing.T) {
	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)
	ref := imgrectestutil.GetReferenceOutput(t, rts.FsLib, IMG_BUCKET, IMG_KEY, MODEL_BUCKET, MODEL_KEY, KID)

	conf := imgrec_py.NewImgrecPyJobConfig(IMG_BUCKET, IMG_KEY, MODEL_BUCKET, MODEL_KEY, KID, false, false, 0, 0, "")
	job, err := imgrec_py.NewImgrecPyJob(conf, rts.SigmaClnt)
	if !assert.Nil(t, err, "NewImgrecPyJob: %v", err) {
		return
	}

	msg, err := job.Run(sp.NOT_SET)
	assert.Nil(t, err, "Run: %v", err)
	db.DPrintf(db.TEST, "imgrec pred: %v", msg)
	imgrectestutil.AssertMatchesReference(t, msg, ref)
}

func TestImgrecPyAsync(t *testing.T) {
	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)
	ref := imgrectestutil.GetReferenceOutput(t, rts.FsLib, IMG_BUCKET, IMG_KEY, MODEL_BUCKET, MODEL_KEY, KID)

	conf := imgrec_py.NewImgrecPyJobConfig(IMG_BUCKET, IMG_KEY, MODEL_BUCKET, MODEL_KEY, KID, false, true, 0, 0, "")
	job, err := imgrec_py.NewImgrecPyJob(conf, rts.SigmaClnt)
	if !assert.Nil(t, err, "NewImgrecPyJob: %v", err) {
		return
	}

	msg, err := job.Run(sp.NOT_SET)
	assert.Nil(t, err, "Run: %v", err)
	db.DPrintf(db.TEST, "imgrec pred: %v", msg)
	imgrectestutil.AssertMatchesReference(t, msg, ref)
}

func TestImgrecPyCoSandbox(t *testing.T) {
	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)
	ref := imgrectestutil.GetReferenceOutput(t, rts.FsLib, IMG_BUCKET, IMG_KEY, MODEL_BUCKET, MODEL_KEY, KID)

	conf := imgrec_py.NewImgrecPyJobConfig(IMG_BUCKET, IMG_KEY, MODEL_BUCKET, MODEL_KEY, KID, true, false, 0, 0, "")
	job, err := imgrec_py.NewImgrecPyJob(conf, rts.SigmaClnt)
	if !assert.Nil(t, err, "NewImgrecPyJob: %v", err) {
		return
	}

	msg, err := job.Run(sp.NOT_SET)
	assert.Nil(t, err, "Run: %v", err)
	db.DPrintf(db.TEST, "imgrec pred: %v", msg)
	imgrectestutil.AssertMatchesReference(t, msg, ref)
}

func TestImgrecPyShmem(t *testing.T) {
	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)
	ref := imgrectestutil.GetReferenceOutput(t, rts.FsLib, IMG_BUCKET, IMG_KEY, MODEL_BUCKET, MODEL_KEY, KID)

	conf := imgrec_py.NewImgrecPyJobConfig(IMG_BUCKET, IMG_KEY, MODEL_BUCKET, MODEL_KEY, KID, true, false, proc.Tmem(256), 0, "")
	job, err := imgrec_py.NewImgrecPyJob(conf, rts.SigmaClnt)
	if !assert.Nil(t, err, "NewImgrecPyJob: %v", err) {
		return
	}

	msg, err := job.Run(sp.NOT_SET)
	assert.Nil(t, err, "Run: %v", err)
	db.DPrintf(db.TEST, "imgrec pred: %v", msg)
	imgrectestutil.AssertMatchesReference(t, msg, ref)
}

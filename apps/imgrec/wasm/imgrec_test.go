package imgrec_wasm_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	imgrectestutil "sigmaos/apps/imgrec/testutil"
	imgrec_wasm "sigmaos/apps/imgrec/wasm"
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

func TestCompile(t *testing.T) {
}

func TestImgrecNoCS(t *testing.T) {
	mrts, err1 := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)
	ref := imgrectestutil.GetReferenceOutput(t, rts.FsLib, IMG_BUCKET, IMG_KEY, MODEL_BUCKET, MODEL_KEY, KID)

	conf := imgrec_wasm.NewImgrecWASMJobConfig(IMG_BUCKET, IMG_KEY, MODEL_BUCKET, MODEL_KEY, KID, false, false, 0, false, 0)
	job, err := imgrec_wasm.NewImgrecWASMJob(conf, rts.SigmaClnt)
	if !assert.Nil(t, err, "NewImgrecWASMJob: %v", err) {
		return
	}

	msg, err := job.Run(sp.NOT_SET)
	assert.Nil(t, err, "Run: %v", err)
	db.DPrintf(db.TEST, "imgrec pred: %v", msg)
	imgrectestutil.AssertMatchesReference(t, msg, ref)
}

func TestImgrecWASMCoSandboxShmem(t *testing.T) {
	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)
	ref := imgrectestutil.GetReferenceOutput(t, rts.FsLib, IMG_BUCKET, IMG_KEY, MODEL_BUCKET, MODEL_KEY, KID)

	conf := imgrec_wasm.NewImgrecWASMJobConfig(IMG_BUCKET, IMG_KEY, MODEL_BUCKET, MODEL_KEY, KID, true, true, proc.Tmem(32), true, 0)
	job, err := imgrec_wasm.NewImgrecWASMJob(conf, rts.SigmaClnt)
	if !assert.Nil(t, err, "NewImgrecWASMJob: %v", err) {
		return
	}

	msg, err := job.Run(sp.NOT_SET)
	assert.Nil(t, err, "Run: %v", err)
	db.DPrintf(db.TEST, "imgrec pred: %v", msg)
	imgrectestutil.AssertMatchesReference(t, msg, ref)
}

func TestImgrecWASMCoSandboxVanilla(t *testing.T) {
	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err, "Error New Tstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)
	ref := imgrectestutil.GetReferenceOutput(t, rts.FsLib, IMG_BUCKET, IMG_KEY, MODEL_BUCKET, MODEL_KEY, KID)

	conf := imgrec_wasm.NewImgrecWASMJobConfig(IMG_BUCKET, IMG_KEY, MODEL_BUCKET, MODEL_KEY, KID, true, true, 0, false, 0)
	job, err := imgrec_wasm.NewImgrecWASMJob(conf, rts.SigmaClnt)
	if !assert.Nil(t, err, "NewImgrecWASMJob: %v", err) {
		return
	}

	msg, err := job.Run(sp.NOT_SET)
	assert.Nil(t, err, "Run: %v", err)
	db.DPrintf(db.TEST, "imgrec pred: %v", msg)
	imgrectestutil.AssertMatchesReference(t, msg, ref)
}

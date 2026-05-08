package sebs_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	sebs "sigmaos/apps/sebs"
	"sigmaos/proc"
	s3clnt "sigmaos/proxy/s3/clnt"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

const (
	SEBS_BUCKET   = "9ps3"
	SEBS_KID      = "~local"
	INPUT_BASE    = "serverless-benchmarks-input"
	SEBS_DATA_DIR = "/tmp/sebs-data"
	SEBS_SHMEM_MB = proc.Tmem(256)
)

func sebsS3Clnt(t *testing.T, rts *test.RealmTstate) *s3clnt.S3Clnt {
	t.Helper()
	pn := "name/s3/" + SEBS_KID
	sc, err := s3clnt.NewS3Clnt(rts.FsLib, pn)
	if !assert.Nil(t, err, "NewS3Clnt: %v", err) {
		t.FailNow()
	}
	return sc
}

func sebsRun(t *testing.T, benchmark, eventJSON string) map[string]any {
	t.Helper()
	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err, "NewMultiRealmTstate: %v", err) {
		return nil
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)
	conf := sebs.NewSebsJobConfig(benchmark, eventJSON, false, false, false, SEBS_SHMEM_MB, 0)
	job, err := sebs.NewSebsJob(conf, rts.SigmaClnt)
	if !assert.Nil(t, err, "NewSebsJob: %v", err) {
		return nil
	}
	msg, err := job.Run(sp.NOT_SET)
	if !assert.Nil(t, err, "Run: %v", err) {
		return nil
	}
	var result map[string]any
	if !assert.Nil(t, json.Unmarshal([]byte(msg), &result), "unmarshal result") {
		return nil
	}
	return result
}

func TestMain(m *testing.M) {
	if _, err := os.Stat(SEBS_DATA_DIR); os.IsNotExist(err) {
		scriptPath := filepath.Join("..", "..", "scripts", "download-sebs-data.py")
		cmd := exec.Command("python3", scriptPath, "--data-dir", SEBS_DATA_DIR)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to download sebs data: %v\n", err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

func TestCompile(t *testing.T) {
}

func TestSebsSleep(t *testing.T) {
	event := `{"sleep": 1}`
	result := sebsRun(t, "010.sleep", event)
	if result == nil {
		return
	}
	assert.Equal(t, float64(1), result["result"], "sleep result mismatch")
}

func TestSebsDynamicHtml(t *testing.T) {
	event := `{"username": "testname", "random_len": 10}`
	result := sebsRun(t, "110.dynamic-html", event)
	if result == nil {
		return
	}
	html, ok := result["result"].(string)
	assert.True(t, ok && len(html) > 0, "result should be non-empty HTML string")
	assert.True(t, strings.Contains(html, "testname"), "HTML should contain username")
}

func TestSebsUploader(t *testing.T) {
	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err, "NewMultiRealmTstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)

	outputPrefix := fmt.Sprintf("%s/120.uploader/output/0", INPUT_BASE)
	eventMap := map[string]any{
		"bucket": map[string]any{
			"bucket": SEBS_BUCKET,
			"output": outputPrefix,
		},
		"object": map[string]any{
			// Stable 230 kB Wikimedia JPEG used by SeBS test size
			"url": "https://upload.wikimedia.org/wikipedia/commons/thumb/e/e7/Jammlich_crop.jpg/800px-Jammlich_crop.jpg",
		},
	}
	eventJSON, _ := json.Marshal(eventMap)

	conf := sebs.NewSebsJobConfig("120.uploader", string(eventJSON), false, false, false, SEBS_SHMEM_MB, 0)
	job, err := sebs.NewSebsJob(conf, rts.SigmaClnt)
	if !assert.Nil(t, err, "NewSebsJob: %v", err) {
		return
	}
	msg, err := job.Run(sp.NOT_SET)
	if !assert.Nil(t, err, "Run: %v", err) {
		return
	}
	var result map[string]any
	assert.Nil(t, json.Unmarshal([]byte(msg), &result))
	res, _ := result["result"].(map[string]any)
	key, _ := res["key"].(string)
	assert.NotEmpty(t, key, "uploader should return non-empty key")
}

func TestSebsThumbnailer(t *testing.T) {
	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err, "NewMultiRealmTstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)
	sc := sebsS3Clnt(t, rts)

	// Upload a minimal test JPEG to S3
	inputPrefix := fmt.Sprintf("%s/210.thumbnailer/input/0", INPUT_BASE)
	outputPrefix := fmt.Sprintf("%s/210.thumbnailer/output/0", INPUT_BASE)
	imgKey := "test.jpg"
	s3Key := fmt.Sprintf("%s/%s", inputPrefix, imgKey)
	if err := sc.PutObject(SEBS_BUCKET, s3Key, makeTestJPEG()); !assert.Nil(t, err, "PutObject: %v", err) {
		return
	}

	eventMap := map[string]any{
		"bucket": map[string]any{
			"bucket": SEBS_BUCKET,
			"input":  inputPrefix,
			"output": outputPrefix,
		},
		"object": map[string]any{
			"key":    imgKey,
			"width":  200,
			"height": 200,
		},
	}
	eventJSON, _ := json.Marshal(eventMap)

	conf := sebs.NewSebsJobConfig("210.thumbnailer", string(eventJSON), false, false, false, SEBS_SHMEM_MB, 0)
	job, err := sebs.NewSebsJob(conf, rts.SigmaClnt)
	if !assert.Nil(t, err, "NewSebsJob: %v", err) {
		return
	}
	msg, err := job.Run(sp.NOT_SET)
	if !assert.Nil(t, err, "Run: %v", err) {
		return
	}
	var result map[string]any
	assert.Nil(t, json.Unmarshal([]byte(msg), &result))
	res, _ := result["result"].(map[string]any)
	key, _ := res["key"].(string)
	assert.NotEmpty(t, key, "thumbnailer should return non-empty key")
}

func TestSebsVideoProcessing(t *testing.T) {
	dataDir := SEBS_DATA_DIR
	videoPath := filepath.Join(dataDir, "220.video-processing", "city.mp4")
	if dataDir == "" || !fileExists(videoPath) {
		t.Skipf("TestSebsVideoProcessing: set SEBS_DATA_DIR with city.mp4 to enable (missing: %v)", videoPath)
	}

	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err, "NewMultiRealmTstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)
	sc := sebsS3Clnt(t, rts)

	inputPrefix := fmt.Sprintf("%s/220.video-processing/input/0", INPUT_BASE)
	outputPrefix := fmt.Sprintf("%s/220.video-processing/output/0", INPUT_BASE)
	videoData, err := os.ReadFile(videoPath)
	if !assert.Nil(t, err, "ReadFile: %v", err) {
		return
	}
	s3Key := fmt.Sprintf("%s/city.mp4", inputPrefix)
	if err := sc.PutObject(SEBS_BUCKET, s3Key, videoData); !assert.Nil(t, err, "PutObject: %v", err) {
		return
	}

	eventMap := map[string]any{
		"bucket": map[string]any{
			"bucket": SEBS_BUCKET,
			"input":  inputPrefix,
			"output": outputPrefix,
		},
		"object": map[string]any{
			"key":      "city.mp4",
			"op":       "watermark",
			"duration": 1,
		},
	}
	eventJSON, _ := json.Marshal(eventMap)

	conf := sebs.NewSebsJobConfig("220.video-processing", string(eventJSON), false, false, false, SEBS_SHMEM_MB, 0)
	job, err := sebs.NewSebsJob(conf, rts.SigmaClnt)
	if !assert.Nil(t, err, "NewSebsJob: %v", err) {
		return
	}
	msg, err := job.Run(sp.NOT_SET)
	if !assert.Nil(t, err, "Run: %v", err) {
		return
	}
	var result map[string]any
	assert.Nil(t, json.Unmarshal([]byte(msg), &result))
	res, _ := result["result"].(map[string]any)
	key, _ := res["key"].(string)
	assert.NotEmpty(t, key, "video-processing should return non-empty key")
}

func TestSebsCompression(t *testing.T) {
	t.Log("TestSebsCompression skipped: download_directory requires S3 list-objects, not yet supported by SigmaOS S3 API")
	t.Skip()
}

func TestSebsImageRecognition(t *testing.T) {
	dataDir := SEBS_DATA_DIR
	modelPath := filepath.Join(dataDir, "411.image-recognition", "model", "resnet50-19c8e357.pth")
	imgPath := filepath.Join(dataDir, "411.image-recognition", "fake-resnet", "800px-Porsche_991_silver_IAA.jpg")
	if dataDir == "" || !fileExists(modelPath) || !fileExists(imgPath) {
		t.Skipf("TestSebsImageRecognition: set SEBS_DATA_DIR with model and image to enable")
	}

	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err, "NewMultiRealmTstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)
	sc := sebsS3Clnt(t, rts)

	modelPrefix := fmt.Sprintf("%s/411.image-recognition/input/0", INPUT_BASE)
	inputPrefix := fmt.Sprintf("%s/411.image-recognition/input/1", INPUT_BASE)
	modelKey := "resnet50-19c8e357.pth"
	imgKey := "800px-Porsche_991_silver_IAA.jpg"

	modelData, err := os.ReadFile(modelPath)
	if !assert.Nil(t, err, "ReadFile model: %v", err) {
		return
	}
	imgData, err := os.ReadFile(imgPath)
	if !assert.Nil(t, err, "ReadFile image: %v", err) {
		return
	}
	if err := sc.PutObject(SEBS_BUCKET, fmt.Sprintf("%s/%s", modelPrefix, modelKey), modelData); !assert.Nil(t, err) {
		return
	}
	if err := sc.PutObject(SEBS_BUCKET, fmt.Sprintf("%s/%s", inputPrefix, imgKey), imgData); !assert.Nil(t, err) {
		return
	}

	eventMap := map[string]any{
		"bucket": map[string]any{
			"bucket": SEBS_BUCKET,
			"model":  modelPrefix,
			"input":  inputPrefix,
		},
		"object": map[string]any{
			"model": modelKey,
			"input": imgKey,
		},
	}
	eventJSON, _ := json.Marshal(eventMap)

	conf := sebs.NewSebsJobConfig("411.image-recognition", string(eventJSON), false, false, false, SEBS_SHMEM_MB, 0)
	job, err := sebs.NewSebsJob(conf, rts.SigmaClnt)
	if !assert.Nil(t, err, "NewSebsJob: %v", err) {
		return
	}
	msg, err := job.Run(sp.NOT_SET)
	if !assert.Nil(t, err, "Run: %v", err) {
		return
	}
	var result map[string]any
	assert.Nil(t, json.Unmarshal([]byte(msg), &result))
	res, _ := result["result"].(map[string]any)
	assert.Equal(t, "sports_car", res["class"], "image-recognition class mismatch")
	assert.Equal(t, float64(817), res["idx"], "image-recognition idx mismatch")
}

func TestSebsGraphPagerank(t *testing.T) {
	event := `{"size": 10, "seed": 42}`
	result := sebsRun(t, "501.graph-pagerank", event)
	if result == nil {
		return
	}
	_, ok := result["result"].(float64)
	assert.True(t, ok, "pagerank result should be a float")
}

func TestSebsGraphMst(t *testing.T) {
	event := `{"size": 10, "seed": 42}`
	result := sebsRun(t, "502.graph-mst", event)
	if result == nil {
		return
	}
	assert.NotNil(t, result["result"], "mst result should be non-nil")
}

func TestSebsGraphBfs(t *testing.T) {
	event := `{"size": 10, "seed": 42}`
	result := sebsRun(t, "503.graph-bfs", event)
	if result == nil {
		return
	}
	bfs, ok := result["result"].([]any)
	assert.True(t, ok && len(bfs) == 3, "bfs result should be a 3-element array")
}

func TestSebsDnaVisualisation(t *testing.T) {
	mrts, err := test.NewMultiRealmTstate(t, []sp.Trealm{test.REALM1})
	if !assert.Nil(t, err, "NewMultiRealmTstate: %v", err) {
		return
	}
	defer mrts.Shutdown()

	rts := mrts.GetRealm(test.REALM1)
	sc := sebsS3Clnt(t, rts)

	inputPrefix := fmt.Sprintf("%s/504.dna-visualisation/input/0", INPUT_BASE)
	outputPrefix := fmt.Sprintf("%s/504.dna-visualisation/output/0", INPUT_BASE)
	fastaKey := "test.fasta"
	s3Key := fmt.Sprintf("%s/%s", inputPrefix, fastaKey)
	if err := sc.PutObject(SEBS_BUCKET, s3Key, minimalFASTA()); !assert.Nil(t, err, "PutObject: %v", err) {
		return
	}

	eventMap := map[string]any{
		"bucket": map[string]any{
			"bucket": SEBS_BUCKET,
			"input":  inputPrefix,
			"output": outputPrefix,
		},
		"object": map[string]any{
			"key": fastaKey,
		},
	}
	eventJSON, _ := json.Marshal(eventMap)

	conf := sebs.NewSebsJobConfig("504.dna-visualisation", string(eventJSON), false, false, false, SEBS_SHMEM_MB, 0)
	job, err := sebs.NewSebsJob(conf, rts.SigmaClnt)
	if !assert.Nil(t, err, "NewSebsJob: %v", err) {
		return
	}
	msg, err := job.Run(sp.NOT_SET)
	if !assert.Nil(t, err, "Run: %v", err) {
		return
	}
	var result map[string]any
	assert.Nil(t, json.Unmarshal([]byte(msg), &result))
	res, _ := result["result"].(map[string]any)
	key, _ := res["key"].(string)
	assert.NotEmpty(t, key, "dna-visualisation should return non-empty key")
}

// makeTestJPEG returns a small valid JPEG image (100x100 solid color).
func makeTestJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

// minimalFASTA returns a minimal valid FASTA file for squiggle.
func minimalFASTA() []byte {
	seq := strings.Repeat("ATCGATCGATCG", 20) // 240 bases
	return []byte(fmt.Sprintf(">test_seq\n%s\n", seq))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

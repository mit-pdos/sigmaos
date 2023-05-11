package imgresizesrv_test

import (
	"fmt"
	"image/jpeg"
	"os"
	"path"
	"testing"
	"time"

	"github.com/nfnt/resize"
	"github.com/stretchr/testify/assert"

	// db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/test"
)

func TestResizeImg(t *testing.T) {
	fn := "/tmp/thumb.jpeg"

	os.Remove(fn)

	in, err := os.Open("/home/kaashoek/Downloads/desk.jpg")
	assert.Nil(t, err)
	img, err := jpeg.Decode(in)
	assert.Nil(t, err)

	start := time.Now()

	img1 := resize.Resize(160, 0, img, resize.Lanczos3)

	fmt.Printf("resize %v\v", time.Since(start))

	out, err := os.Create(fn)
	assert.Nil(t, err)
	jpeg.Encode(out, img1, nil)
}

func TestResizeProc(t *testing.T) {
	ts := test.MakeTstateAll(t)
	in := path.Join(sp.S3, "~local/9ps3/desk.jpg")
	out := path.Join(sp.S3, "~local/9ps3/thumb.jpg")
	ts.Remove(out)
	p := proc.MakeProc("imgresize", []string{in, out})
	err := ts.Spawn(p)
	assert.Nil(t, err, "Spawn")
	err = ts.WaitStart(p.GetPid())
	assert.Nil(t, err, "WaitStart error")
	status, err := ts.WaitExit(p.GetPid())
	assert.True(t, status.IsStatusOK(), "WaitExit status error")
	ts.Shutdown()
}

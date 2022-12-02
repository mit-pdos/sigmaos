package npcodec_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	np "sigmaos/sigmap"
	"sigmaos/npcodec"
)

func TestPutfile(t *testing.T) {
	b := []byte("hello")
	fence := &np.Tfence{Fenceid: &np.Tfenceid{Path: 36, Serverid: 2}, Epoch: 7}
	msg := &np.Tputfile{1, np.OWRITE, 0777, 101, []string{"f"}, b}
	fcall := np.MakeFcallMsg(msg, 0, 13, nil, &np.Tinterval{Start: 1, End: 2}, fence)
	frame, error := npcodec.MarshalFcallMsgByte(fcall)
	assert.Nil(t, error)
	fcall1, error := npcodec.UnmarshalFcallMsg(frame)
	assert.Nil(t, error)
	assert.Equal(t, fcall1.Fc.Type, fcall.Fc.Type, "type")
	assert.Equal(t, fcall1.Fc.Fence.Fenceid.Path, fcall.Fc.Fence.Fenceid.Path, "path")
	assert.Equal(t, fcall1.Fc.Fence.Fenceid.Serverid, fcall.Fc.Fence.Fenceid.Serverid, "id")
	assert.Equal(t, fcall1.Fc.Fence.Epoch, fcall.Fc.Fence.Epoch, "epoch")
	assert.Equal(t, fcall1.Fc.Received.Start, fcall.Fc.Received.Start, "start")
	assert.Equal(t, fcall1.Fc.Received.End, fcall.Fc.Received.End, "end")
	assert.Equal(t, *fcall1.GetMsg().(*np.Tputfile), *fcall.GetMsg().(*np.Tputfile), "fcall")
}

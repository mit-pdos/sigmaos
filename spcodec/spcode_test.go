package spcodec

import (
	"testing"

	"github.com/stretchr/testify/assert"

	sp "sigmaos/sigmap"
)

func TestPutfile(t *testing.T) {
	b := []byte("hello")
	fence := &sp.Tfence{Fenceid: &sp.Tfenceid{Path: 36, Serverid: 2}, Epoch: 7}
	msg := sp.MkTputfile(1, sp.OWRITE, 0777, 101, []string{"f"}, b)
	fcall := sp.MakeFcallMsg(msg, 0, 13, nil, &sp.Tinterval{Start: 1, End: 2}, fence)
	frame, error := MarshalFcallMsgByte(fcall)
	assert.Nil(t, error)
	fc, error := UnmarshalFcallMsg(frame)
	assert.Nil(t, error)
	fcall1 := fc.(*sp.FcallMsg)
	assert.Equal(t, fcall1.Fc.Type, fcall.Fc.Type, "type")
	assert.Equal(t, fcall1.Fc.Fence.Fenceid.Path, fcall.Fc.Fence.Fenceid.Path, "path")
	assert.Equal(t, fcall1.Fc.Fence.Fenceid.Serverid, fcall.Fc.Fence.Fenceid.Serverid, "id")
	assert.Equal(t, fcall1.Fc.Fence.Epoch, fcall.Fc.Fence.Epoch, "epoch")
	assert.Equal(t, fcall1.Fc.Received.Start, fcall.Fc.Received.Start, "start")
	assert.Equal(t, fcall1.Fc.Received.End, fcall.Fc.Received.End, "end")
	msg1 := *fcall1.GetMsg().(*sp.Tputfile)
	assert.Equal(t, msg.Wnames, msg1.Wnames, "fcall")
}

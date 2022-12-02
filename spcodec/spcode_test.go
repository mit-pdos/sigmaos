package spcodec

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	sp "sigmaos/sigmap"
)

func TestQid(t *testing.T) {
	qid := sp.MakeQid(0, 3, 2)
	b, err := marshal(qid)
	assert.Nil(t, err)
	st1 := sp.Tqid{}
	err = unmarshal(b, &st1)
	assert.Nil(t, err)
	fmt.Printf("qid1 %v\n", st1)
}

func TestStat(t *testing.T) {
	st := sp.MkStat(sp.MakeQid(0, 0, 2), 0, 0, "alice", "bob")
	st.Length = 10
	b, err := marshal(st)
	assert.Nil(t, err)
	fmt.Printf("st %v\n", string(b))
	st1 := sp.Stat{}
	err = unmarshal(b, &st1)
	assert.Nil(t, err)
	fmt.Printf("st1 %v\n", st1)
	assert.Equal(t, "alice", st1.Name)
}

func TestPutfile(t *testing.T) {
	b := []byte("hello")
	fence := &sp.Tfence{Fenceid: &sp.Tfenceid{Path: 36, Serverid: 2}, Epoch: 7}
	msg := &sp.Tputfile{1, sp.OWRITE, 0777, 101, []string{"f"}, b}
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
	assert.Equal(t, *fcall1.GetMsg().(*sp.Tputfile), *fcall.GetMsg().(*sp.Tputfile), "fcall")
}

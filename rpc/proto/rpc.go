package proto

import (
	sessp "sigmaos/session/proto"
)

func NewBlob(iov *sessp.IoVec) *Blob {
	bl := &Blob{}
	bl.SetIoVec(iov)
	return bl
}

func (bl *Blob) GetIoVec() *sessp.IoVec {
	bs := bl.GetIov()
	return sessp.NewIoVec(bs, nil)
}

func (bl *Blob) SetIoVec(iov *sessp.IoVec) {
	// Bail out early if no IoVec is set for this RPC
	if iov == nil {
		return
	}
	bs := make([][]byte, iov.Len())
	for i := range bs {
		bs[i] = iov.GetFrame(i).GetBuf()
	}
	bl.Iov = bs
}

func (bl *Blob) SetIoVecBufs(bs [][]byte) {
	bl.Iov = make([][]byte, len(bs))
	for i := range bs {
		bl.Iov[i] = bs[i]
	}
	bl.Iov = bs
}

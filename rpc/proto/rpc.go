package proto

import (
	sessp "sigmaos/session/proto"
)

func NewBlob(iov sessp.IoVec) *Blob {
	bl := &Blob{}
	bl.SetIoVec(iov)
	return bl
}

func (bl *Blob) GetIoVec() sessp.IoVec {
	iov := bl.GetIov()
	return sessp.NewIoVec(iov)
}

func (bl *Blob) SetIoVec(iov sessp.IoVec) {
	blob := make([][]byte, len(iov))
	for i := 0; i < len(iov); i++ {
		blob[i] = iov[i]
	}
	bl.Iov = blob
}

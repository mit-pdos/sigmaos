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
	// If this blob is composed of split IOVecs, construct an IOVec from the
	// splits
	if len(bl.GetSplitIov()) > 0 {
		siov := bl.GetSplitIov()
		fs := make([][][]byte, len(siov))
		for i := range siov {
			fs[i] = siov[i].Iov
		}
		return sessp.NewMultiBufIoVec(fs)
	}
	bs := bl.GetIov()
	return sessp.NewIoVec(bs, nil)
}

func (bl *Blob) ClearIoVec() {
	bl.Iov = nil
	bl.SplitIov = nil
}

func (bl *Blob) SetIoVec(iov *sessp.IoVec) {
	// Bail out early if no IoVec is set for this RPC
	if iov == nil {
		return
	}
	if !iov.GetIsMultiBuf() {
		bs := make([][]byte, iov.Len())
		for i := range bs {
			bs[i] = iov.GetFrame(i).GetBuf()
		}
		bl.Iov = bs
	} else {
		splitIovs := make([]*SplitIoVec, iov.Len())
		for i, f := range iov.GetFrames() {
			splitIovs[i] = &SplitIoVec{
				Iov: f.GetMultiBuf(),
			}
		}
		bl.SplitIov = splitIovs
	}
}

func (bl *Blob) SetIoVecBufs(bs [][]byte) {
	bl.Iov = make([][]byte, len(bs))
	for i := range bs {
		bl.Iov[i] = bs[i]
	}
	bl.Iov = bs
}

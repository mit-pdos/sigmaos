package frame

import (
	"encoding/binary"
	"io"

	"time"

	db "sigmaos/debug"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
)

// Read a frame into an existing buffer
func ReadFrameInto(rd io.Reader, frame *sessp.Tframe) error {
	var nb uint32
	if err := binary.Read(rd, binary.LittleEndian, &nb); err != nil {
		return err
	}
	nbyte := int(nb)
	if nbyte < 4 {
		db.DPrintf(db.FRAME, "[%p] Error ReadFrameInto nbyte too short %d", rd, nbyte)
		return io.ErrShortBuffer
	}
	nbyte = nbyte - 4
	db.DPrintf(db.FRAME, "[%p] ReadFrameInto nbyte %d", rd, nbyte)
	// If no frame to read into was specified, allocate one
	if !frame.IsAllocated() {
		frame.GrowBuf(nbyte)
	}
	if nbyte > frame.Len() {
		db.DFatalf("Output buf too small: %v < %v", frame.Len(), nbyte)
	}
	// Only read the first nbyte bytes
	frame.TruncateBuf(nbyte)
	n, e := io.ReadFull(rd, frame.GetBuf())
	if n != nbyte {
		return io.ErrShortBuffer
	}
	if e != nil {
		return e
	}
	return nil
}

// Read a specific number of frames into pre-existing buffers
func ReadNFramesInto(rd io.Reader, iov *sessp.IoVec) error {
	for i := 0; i < iov.Len(); i++ {
		err := ReadFrameInto(rd, iov.GetFrame(i))
		if err != nil {
			return err
		}
	}
	return nil
}

// Read a single frame, constructing the necessary buffer to receive it
func ReadFrame(rd io.Reader) (*sessp.Tframe, error) {
	f := sessp.NewUnallocatedFrame()
	return f, ReadFrameInto(rd, f)
}

// Read many frames, constructing the necessary buffers to receive them
func ReadFrames(rd io.Reader) (*sessp.IoVec, error) {
	nframes, err := ReadNumOfFrames(rd)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.FRAME, "[%p] ReadFrames %d", rd, nframes)
	if nframes < 0 {
		return nil, io.ErrShortBuffer
	}
	iov := sessp.NewUnallocatedIoVec(int(nframes), nil)
	if err := ReadNFramesInto(rd, iov); err != nil {
		return nil, err
	}
	return iov, nil
}

// Write a single frame's buffer
func WriteFrame(wr io.Writer, frame *sessp.Tframe) error {
	return WriteFrameBuf(wr, frame.GetBuf())
}

// Write a single frame
func WriteFrameBuf(wr io.Writer, b []byte) error {
	nbyte := uint32(len(b) + 4) // +4 because that is how 9P wants it
	db.DPrintf(db.FRAME, "[%p] WriteFrame nbyte %v %v", wr, len(b), nbyte)
	if err := binary.Write(wr, binary.LittleEndian, nbyte); err != nil {
		return err
	}
	return writeRawBuffer(wr, b)
}

// Write many frames
func WriteFrames(wr io.Writer, iov *sessp.IoVec) error {
	db.DPrintf(db.FRAME, "[%p] WriteFrames %d", wr, iov.Len())
	if err := WriteNumOfFrames(wr, uint32(iov.Len())); err != nil {
		return err
	}
	start := time.Now()
	nbyte := 0
	for _, f := range iov.GetFrames() {
		start := time.Now()
		if err := WriteFrame(wr, f); err != nil {
			return err
		}
		nbyte += f.Len()
		if db.WillBePrinted(db.PROXY_RPC_LAT) && f.Len() > 2*sp.MBYTE {
			db.DPrintf(db.PROXY_RPC_LAT, "Done write %vB lat=%v tpt=%0.3fMB/s", f.Len(), time.Since(start), (float64(f.Len())/time.Since(start).Seconds())/float64(sp.MBYTE))
		}
	}
	if db.WillBePrinted(db.PROXY_RPC_LAT) && nbyte > 2*sp.MBYTE {
		db.DPrintf(db.PROXY_RPC_LAT, "Done write all iov %vB lat=%v tpt=%0.3fMB/s", nbyte, time.Since(start), (float64(nbyte)/time.Since(start).Seconds())/float64(sp.MBYTE))
	}
	return nil
}

// Write a raw buffer over the wire
func writeRawBuffer(wr io.Writer, b []byte) error {
	if nbytes, err := wr.Write(b); err != nil {
		return err
	} else if nbytes < len(b) {
		return io.ErrShortWrite
	}
	return nil
}

// Write the sequence number of the next RPC to be written
func WriteSeqno(seqno sessp.Tseqno, wr io.Writer) error {
	sn := uint64(seqno)
	if err := binary.Write(wr, binary.LittleEndian, sn); err != nil {
		return err
	}
	return nil
}

// Read the sequence number of the next RPC to be read
func ReadSeqno(rdr io.Reader) (sessp.Tseqno, error) {
	var sn uint64
	if err := binary.Read(rdr, binary.LittleEndian, &sn); err != nil {
		return 0, err
	}
	return sessp.Tseqno(sn), nil
}

// Read the number of frames that follow
func ReadNumOfFrames(rd io.Reader) (uint32, error) {
	var nframes uint32
	if err := binary.Read(rd, binary.LittleEndian, &nframes); err != nil {
		return 0, err
	}
	return nframes, nil
}

// Write the number of frames that follow
func WriteNumOfFrames(wr io.Writer, nframes uint32) error {
	if err := binary.Write(wr, binary.LittleEndian, nframes); err != nil {
		return err
	}
	return nil
}

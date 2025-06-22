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
	var nbyte uint32
	if err := binary.Read(rd, binary.LittleEndian, &nbyte); err != nil {
		return err
	}
	if nbyte < 4 {
		db.DPrintf(db.FRAME, "[%p] Error ReadFrameInto nbyte too short %d", rd, nbyte)
		return io.ErrShortBuffer
	}
	nbyte = nbyte - 4
	db.DPrintf(db.FRAME, "[%p] ReadFrameInto nbyte %d", rd, nbyte)
	// If no frame to read into was specified, allocate one
	if *frame == nil {
		*frame = make(sessp.Tframe, nbyte)
	} else {
	}
	if nbyte > uint32(len(*frame)) {
		db.DFatalf("Output buf too smal: %v < %v", len(*frame), nbyte)
	}
	// Only read the first nbyte bytes
	*frame = (*frame)[:nbyte]
	n, e := io.ReadFull(rd, *frame)
	if n != int(nbyte) {
		return io.ErrShortBuffer
	}
	if e != nil {
		return e
	}
	return nil
}

// Read a specific number of frames into pre-existing buffers
func ReadNFramesInto(rd io.Reader, iov sessp.IoVec) error {
	for i := 0; i < len(iov); i++ {
		err := ReadFrameInto(rd, &iov[i])
		if err != nil {
			return err
		}
	}
	return nil
}

// Read a single frame, constructing the necessary buffer to receive it
func ReadFrame(rd io.Reader) (sessp.Tframe, error) {
	var f sessp.Tframe = nil
	return f, ReadFrameInto(rd, &f)
}

// Read many frames, constructing the necessary buffers to receive them
func ReadFrames(rd io.Reader) (sessp.IoVec, error) {
	nframes, err := ReadNumOfFrames(rd)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.FRAME, "[%p] ReadFrames %d", rd, nframes)
	if nframes < 0 {
		return nil, io.ErrShortBuffer
	}
	iov := make(sessp.IoVec, nframes)
	if err := ReadNFramesInto(rd, iov); err != nil {
		return nil, err
	}
	return iov, nil
}

// Write a single frame
func WriteFrame(wr io.Writer, frame sessp.Tframe) error {
	db.DPrintf(db.FRAME, "[%p] WriteFrame nbyte %v %v", wr, len(frame), uint32(len(frame)+4))
	nbyte := uint32(len(frame) + 4) // +4 because that is how 9P wants it
	if err := binary.Write(wr, binary.LittleEndian, nbyte); err != nil {
		return err
	}
	return writeRawBuffer(wr, frame)
}

// Write many frames
func WriteFrames(wr io.Writer, iov sessp.IoVec) error {
	db.DPrintf(db.FRAME, "[%p] WriteFrames %d", wr, len(iov))
	if err := WriteNumOfFrames(wr, uint32(len(iov))); err != nil {
		return err
	}
	start := time.Now()
	nbyte := 0
	for _, f := range iov {
		start := time.Now()
		if err := WriteFrame(wr, f); err != nil {
			return err
		}
		nbyte += len(f)
		if db.WillBePrinted(db.PROXY_RPC_LAT) && len(f) > 2*sp.MBYTE {
			db.DPrintf(db.PROXY_RPC_LAT, "Done write %vB lat=%v tpt=%0.3fMB/s", len(f), time.Since(start), (float64(len(f))/time.Since(start).Seconds())/float64(sp.MBYTE))
		}
	}
	if db.WillBePrinted(db.PROXY_RPC_LAT) && nbyte > 2*sp.MBYTE {
		db.DPrintf(db.PROXY_RPC_LAT, "Done write %vB lat=%v tpt=%0.3fMB/s", nbyte, time.Since(start), (float64(nbyte)/time.Since(start).Seconds())/float64(sp.MBYTE))
	}
	return nil
}

// Write a raw buffer over the wire
func writeRawBuffer(wr io.Writer, buf sessp.Tframe) error {
	if nbytes, err := wr.Write(buf); err != nil {
		return err
	} else if nbytes < len(buf) {
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

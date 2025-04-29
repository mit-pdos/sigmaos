package frame

import (
	"encoding/binary"
	"fmt"
	"io"

	db "sigmaos/debug"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
)

// Read a frame into an existing buffer
func ReadFrameInto(rd io.Reader, frame *sessp.Tframe) *serr.Err {
	var nbyte uint32
	if err := binary.Read(rd, binary.LittleEndian, &nbyte); err != nil {
		// EOFs are handled differently from other errors, which indicate the
		// sender is Unreachable
		if err == io.EOF {
			return serr.NewErrError(err)
		}
		return serr.NewErr(serr.TErrUnreachable, err)
	}
	if nbyte < 4 {
		db.DPrintf(db.FRAME, "[%p] Error ReadFrameInto nbyte too short %d", rd, nbyte)
		return serr.NewErr(serr.TErrUnreachable, fmt.Errorf("readMsg too short (%v)", nbyte))
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
		return serr.NewErr(serr.TErrUnreachable, e)
	}
	if e != nil {
		return serr.NewErrError(e)
	}
	return nil
}

// Read a specific number of frames into pre-existing buffers
func ReadNFramesInto(rd io.Reader, iov sessp.IoVec) *serr.Err {
	for i := 0; i < len(iov); i++ {
		err := ReadFrameInto(rd, &iov[i])
		if err != nil {
			return err
		}
	}
	return nil
}

// Read a single frame, constructing the necessary buffer to receive it
func ReadFrame(rd io.Reader) (sessp.Tframe, *serr.Err) {
	var f sessp.Tframe = nil
	return f, ReadFrameInto(rd, &f)
}

// Read many frames, constructing the necessary buffers to receive them
func ReadFrames(rd io.Reader) (sessp.IoVec, *serr.Err) {
	nframes, err := ReadNumOfFrames(rd)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.FRAME, "[%p] ReadFrames %d", rd, nframes)
	if nframes < 0 {
		return nil, serr.NewErr(serr.TErrUnreachable, "ReadFrames too short")
	}
	iov := make(sessp.IoVec, nframes)
	if err := ReadNFramesInto(rd, iov); err != nil {
		return nil, err
	}
	return iov, nil
}

// Write a single frame
func WriteFrame(wr io.Writer, frame sessp.Tframe) *serr.Err {
	db.DPrintf(db.FRAME, "[%p] WriteFrame nbyte %v %v", wr, len(frame), uint32(len(frame)+4))
	nbyte := uint32(len(frame) + 4) // +4 because that is how 9P wants it
	if err := binary.Write(wr, binary.LittleEndian, nbyte); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	return writeRawBuffer(wr, frame)
}

// Write many frames
func WriteFrames(wr io.Writer, iov sessp.IoVec) *serr.Err {
	db.DPrintf(db.FRAME, "[%p] WriteFrames %d", wr, len(iov))
	if err := WriteNumOfFrames(wr, uint32(len(iov))); err != nil {
		return err
	}
	for _, f := range iov {
		if err := WriteFrame(wr, f); err != nil {
			return err
		}
	}
	return nil
}

// Write a raw buffer over the wire
func writeRawBuffer(wr io.Writer, buf sessp.Tframe) *serr.Err {
	if nbytes, err := wr.Write(buf); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	} else if nbytes < len(buf) {
		return serr.NewErr(serr.TErrUnreachable, "writeRawBuffer too short")
	}
	return nil
}

// Write the sequence number of the next RPC to be written
func WriteSeqno(seqno sessp.Tseqno, wr io.Writer) *serr.Err {
	sn := uint64(seqno)
	if err := binary.Write(wr, binary.LittleEndian, sn); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	return nil
}

// Read the sequence number of the next RPC to be read
func ReadSeqno(rdr io.Reader) (sessp.Tseqno, *serr.Err) {
	db.DPrintf(db.PYPROXYSRV, "ReadSeqno called\n")
	var sn uint64
	if err := binary.Read(rdr, binary.LittleEndian, &sn); err != nil {
		return 0, serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	db.DPrintf(db.PYPROXYSRV, "ReadSeqno success: %v\n", sn)
	return sessp.Tseqno(sn), nil
}

// Read the number of frames that follow
func ReadNumOfFrames(rd io.Reader) (uint32, *serr.Err) {
	var nframes uint32
	if err := binary.Read(rd, binary.LittleEndian, &nframes); err != nil {
		return 0, serr.NewErr(serr.TErrUnreachable, err)
	}
	db.DPrintf(db.PYPROXYSRV, "ReadNumOfFrames: %v\n", nframes)
	return nframes, nil
}

// Write the number of frames that follow
func WriteNumOfFrames(wr io.Writer, nframes uint32) *serr.Err {
	if err := binary.Write(wr, binary.LittleEndian, nframes); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	return nil
}

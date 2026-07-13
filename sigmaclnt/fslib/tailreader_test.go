package fslib

// Unit tests for tailChunkReader: the final chunk of a ParallelFileReader
// region, which must deliver [off, end) plus — past end — only up through
// the first newline at or after end-1, reading the tail lazily in doubling
// probes capped at max.

import (
	"bytes"
	"io"
	"testing"

	sp "sigmaos/sigmap"
)

// memPread simulates a file and records the read requests issued.
type memPread struct {
	data  []byte
	reads []sp.Tsize
}

func (mp *memPread) pread(off sp.Toffset, sz sp.Tsize) (io.ReadCloser, error) {
	mp.reads = append(mp.reads, sz)
	if off >= sp.Toffset(len(mp.data)) {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	end := off + sp.Toffset(sz)
	if end > sp.Toffset(len(mp.data)) {
		end = sp.Toffset(len(mp.data))
	}
	return io.NopCloser(bytes.NewReader(mp.data[off:end])), nil
}

func tailRead(t *testing.T, data []byte, off, end, max sp.Toffset) ([]byte, int) {
	t.Helper()
	mp := &memPread{data: data}
	tr := newTailChunkReader(mp.pread, off, end, max)
	b, err := io.ReadAll(tr)
	if err != nil {
		t.Fatalf("tailRead: %v", err)
	}
	return b, len(mp.reads)
}

func TestTailReaderLineEndsAtRegionEnd(t *testing.T) {
	// data[end-1] == '\n': deliver exactly [off, end), nothing past it
	data := []byte("aaa bbb\nccc ddd\nAFTER")
	end := sp.Toffset(bytes.IndexByte(data, '\n')) + 1
	b, nreads := tailRead(t, data, 0, end, end+sp.Toffset(len(data)))
	if string(b) != "aaa bbb\n" {
		t.Errorf("got %q", b)
	}
	if nreads != 1 {
		t.Errorf("nreads %d, want 1 (probe folded into first read)", nreads)
	}
}

func TestTailReaderStraddlingLine(t *testing.T) {
	// The line containing end-1 continues past end: deliver through its
	// newline and no further.
	data := []byte("aaa\nbbbXXXccc\nAFTER\n")
	end := sp.Toffset(7) // inside "bbbXXXccc"
	b, nreads := tailRead(t, data, 0, end, end+64)
	if string(b) != "aaa\nbbbXXXccc\n" {
		t.Errorf("got %q", b)
	}
	if nreads != 1 {
		t.Errorf("nreads %d, want 1 (tail within the folded probe)", nreads)
	}
}

func TestTailReaderDoublingProbes(t *testing.T) {
	// A long straddling line: the folded probe misses the newline, so the
	// reader issues doubling probes until it finds it.
	line := bytes.Repeat([]byte("x"), 64*1024)
	data := append(append([]byte("hdr\n"), line...), '\n')
	data = append(data, []byte("AFTER\n")...)
	end := sp.Toffset(8) // a few bytes into the long line
	max := end + 128*1024
	b, nreads := tailRead(t, data, 0, end, max)
	want := len(data) - len("AFTER\n")
	if len(b) != want || b[len(b)-1] != '\n' {
		t.Errorf("got %d bytes (last %q), want %d ending in newline", len(b), b[len(b)-1], want)
	}
	if nreads < 2 {
		t.Errorf("nreads %d, want >= 2 (doubling probes)", nreads)
	}
}

func TestTailReaderCap(t *testing.T) {
	// No newline before max: deliver exactly [off, max) and stop.
	data := bytes.Repeat([]byte("y"), 4096)
	end := sp.Toffset(100)
	max := sp.Toffset(600)
	b, _ := tailRead(t, data, 0, end, max)
	if len(b) != int(max) {
		t.Errorf("got %d bytes, want %d (capped)", len(b), max)
	}
}

func TestTailReaderFileEOF(t *testing.T) {
	// File ends (without a newline) before the probe cap: deliver up to
	// EOF and terminate.
	data := []byte("aaa\nbbb ccc")
	end := sp.Toffset(6) // inside "bbb ccc"
	b, _ := tailRead(t, data, 0, end, end+4096)
	if string(b) != "aaa\nbbb ccc" {
		t.Errorf("got %q", b)
	}
}

func TestTailReaderNonZeroOffset(t *testing.T) {
	// The chunk body starts mid-file; newline scanning starts at end-1.
	data := []byte("0123456789\nabcdef\nAFTER")
	off := sp.Toffset(5)
	end := sp.Toffset(13) // inside "abcdef"
	b, _ := tailRead(t, data, off, end, end+64)
	if string(b) != "56789\nabcdef\n" {
		t.Errorf("got %q", b)
	}
}
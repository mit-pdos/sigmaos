package getput

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"testing"

	"sigmaos/apps/mr/chunkreader"
	"sigmaos/apps/mr/mr"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

const input = "../../input/pg-dorian_gray.txt"

// TestReaderBoundaryCorrectness runs the mapper's chunk-processing machinery
// (the real chunkreader.DoChunk via ReadChunks) over GetPutReaders — direct
// and delegated — for a file cut into many splits, and checks that the
// emitted word multiset matches a sequential scan exactly. The delegated
// variant simulates the cosandbox: it prefetches exactly the
// mr.SplitReadWindow ranges the coordinator's manifest would request, at
// rpcIdx = split index, and the fake fails if any idx is fetched twice —
// tail extensions must be direct RPCs.
func TestReaderBoundaryCorrectness(t *testing.T) {
	data, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}

	// Ground truth via a sequential scan with the mapper's word scanner
	truth := map[string]int{}
	p := &perf.Perf{}
	ckr0 := chunkreader.NewChunkReader(len(data)+1024, 40, nil, p)
	s0 := &mr.Split{File: "name/ux/~local/in", Offset: 0, Length: sp.Tlength(len(data))}
	_, err = ckr0.DoChunk(bytes.NewReader(data), 0, true, s0, func(f string, scan *bufio.Scanner, emit mr.EmitT) error {
		for scan.Scan() {
			truth[scan.Text()]++
		}
		return scan.Err()
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, cfg := range []struct {
		splitsz, linesz, probesz int
		delegated                bool
	}{
		{8192, 4096, 0, false},   // default probe, direct
		{8192, 4096, 0, true},    // default probe, delegated
		{8192, 4096, 8, true},    // tiny probe: tail extension on nearly every split
		{16384, 16384, 0, true},  // fine-grained shape: linesz = splitsz
		{4096, 1024, 16, false},  // small everything
		{10000, 4096, 100, true}, // unaligned boundaries
	} {
		f := newFakeClnt(data)
		// Build the "cosandbox prefetch": one window per split, in bin
		// order, exactly as the coordinator's manifest requests it.
		splits := []*mr.Split{}
		for start := sp.Toffset(0); start < sp.Toffset(len(data)); start += sp.Toffset(cfg.splitsz) {
			length := sp.Tlength(cfg.splitsz)
			if rest := sp.Tlength(len(data)) - sp.Tlength(start); rest < length {
				length = rest
			}
			splits = append(splits, &mr.Split{File: "name/ux/~local/in", Offset: start, Length: length})
		}
		for i, s := range splits {
			off, body, probe := mr.SplitReadWindow(s, cfg.linesz, cfg.probesz)
			end := min(uint64(off)+uint64(body)+uint64(probe), uint64(len(data)))
			f.delegated[uint64(i)] = data[off:end]
		}

		got := map[string]int{}
		mapf := func(file string, scan *bufio.Scanner, emit mr.EmitT) error {
			for scan.Scan() {
				got[scan.Text()]++
			}
			return scan.Err()
		}
		for i, s := range splits {
			off, body, probe := mr.SplitReadWindow(s, cfg.linesz, cfg.probesz)
			r, err := newGetPutReader(f, s.File, off, body, sp.Tlength(cfg.linesz), probe, cfg.delegated, uint64(i))
			if err != nil {
				t.Fatalf("%+v: newGetPutReader: %v", cfg, err)
			}
			ckr := chunkreader.NewChunkReader(cfg.linesz, 40, nil, p)
			if _, err := ckr.ReadChunks(r, s, mapf); err != nil {
				t.Fatalf("%+v: ReadChunks: %v", cfg, err)
			}
		}

		nbad := 0
		for w, n := range got {
			if n != truth[w] {
				nbad++
				if nbad <= 5 {
					t.Errorf("%+v: %q got %d want %d", cfg, w, n, truth[w])
				}
			}
		}
		for w, n := range truth {
			if _, ok := got[w]; !ok && n > 0 {
				nbad++
				if nbad <= 5 {
					t.Errorf("%+v: %q got 0 want %d", cfg, w, n)
				}
			}
		}
		if nbad > 0 {
			t.Errorf("%+v: %d words with wrong counts", cfg, nbad)
		}
		if cfg.delegated {
			for i := range splits {
				if f.ndeleg[uint64(i)] != 1 {
					t.Errorf("%+v: rpcIdx %d fetched %d times, want exactly 1", cfg, i, f.ndeleg[uint64(i)])
				}
			}
		}
	}
}

// TestReaderFinalChunkTail checks the final chunk's tail behaviour: it is
// fetched lazily (the constructor does no I/O on the direct path), and it
// extends past the split end only when the probe misses the straddling line's
// newline.
func TestReaderFinalChunkTail(t *testing.T) {
	data := []byte("aaaa bbbb\ncccc dddd eeee ffff\ngggg\n")
	// Split ends mid-second-line; probe of 4 bytes misses its newline
	s := &mr.Split{File: "name/ux/~local/in", Offset: 0, Length: 14}
	f := newFakeClnt(data)
	off, body, probe := mr.SplitReadWindow(s, 1024, 4)
	r, err := newGetPutReader(f, s.File, off, body, 1024, probe, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	// The direct path fetches per chunk, so opening the reader costs nothing:
	// this is what lets the mapper's chunk readers overlap fetch with mapping.
	if f.ngets != 0 {
		t.Errorf("constructor did %d gets, want 0 (chunks are fetched lazily)", f.ngets)
	}
	rdr, o, final, err := r.GetChunkReader(64, 60)
	if err != nil || !final || o != 0 {
		t.Fatalf("first chunk: o %v final %v err %v", o, final, err)
	}
	if f.ngets < 2 {
		t.Errorf("expected tail extension gets, got %d total gets", f.ngets)
	}
	b, _ := io.ReadAll(rdr)
	// The chunk must reach through the straddling line's newline (byte 29)
	if len(b) < 30 || b[29] != '\n' {
		t.Errorf("chunk too short to finish straddling line: %d bytes %q", len(b), b)
	}
	if _, _, _, err := r.GetChunkReader(64, 60); err != io.EOF {
		t.Errorf("second chunk: want io.EOF, got %v", err)
	}

	// A probe that covers the newline must need exactly one get
	f2 := newFakeClnt(data)
	_, body2, probe2 := mr.SplitReadWindow(s, 1024, 64)
	r2, err := newGetPutReader(f2, s.File, 0, body2, 1024, probe2, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := r2.GetChunkReader(64, 60); err != nil {
		t.Fatal(err)
	}
	if f2.ngets != 1 {
		t.Errorf("probe covers newline: want 1 get, got %d", f2.ngets)
	}
}

// TestReaderTilesWindow is the point of the direct path: a split's window is
// tiled into sz/offinc chunks, each fetched on its own, so that the mapper's
// concurrent chunk readers overlap fetching with mapping and put several ranged
// gets in flight per split. Serving the whole window as one chunk instead left
// four of the five chunk readers idle (claude-slop/GET_PUT_SLOW.md).
func TestReaderTilesWindow(t *testing.T) {
	const (
		sz     = 64 // what the mapper passes: cap(ckr.buf) = linesz + wordsz
		offinc = 60 // ... minus the word overlap
		body   = 600
	)
	data := bytes.Repeat([]byte("word xyz\n"), 200) // 1800 bytes, newline every 9
	s := &mr.Split{File: "name/ux/~local/in", Offset: 0, Length: body}
	f := newFakeClnt(data)
	off, bd, probe := mr.SplitReadWindow(s, 1024, 16)
	r, err := newGetPutReader(f, s.File, off, bd, 1024, probe, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	offs := []sp.Toffset{}
	nfinal := 0
	for {
		rdr, o, final, err := r.GetChunkReader(sz, offinc)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rdr)
		if len(b) == 0 {
			t.Errorf("chunk at %v is empty", o)
		}
		offs = append(offs, o)
		if final {
			nfinal++
		}
		if len(offs) > 100 {
			t.Fatal("chunk cursor is not advancing")
		}
	}
	// ceil(600/60) = 10 chunks, offsets 0, 60, 120, ...
	if len(offs) != 10 {
		t.Errorf("got %d chunks %v, want 10", len(offs), offs)
	}
	for i, o := range offs {
		if o != sp.Toffset(i*offinc) {
			t.Errorf("chunk %d at offset %v, want %v", i, o, i*offinc)
		}
	}
	if nfinal != 1 {
		t.Errorf("%d final chunks, want exactly 1", nfinal)
	}
	// One get per chunk, so the fetches can overlap; the old whole-window
	// reader did exactly one for the entire split.
	if f.ngets < len(offs) {
		t.Errorf("%d gets for %d chunks: chunks are not fetched individually", f.ngets, len(offs))
	}
}

// The delegated (cosandbox) path keeps the whole-window contract: the store
// holds exactly one reply per rpcIdx, so there is nothing to tile.
func TestReaderDelegatedWholeWindow(t *testing.T) {
	data := bytes.Repeat([]byte("word xyz\n"), 200)
	s := &mr.Split{File: "name/ux/~local/in", Offset: 0, Length: 600}
	f := newFakeClnt(data)
	off, body, probe := mr.SplitReadWindow(s, 1024, 16)
	f.delegated[0] = data[off : uint64(off)+uint64(body)+uint64(probe)]
	r, err := newGetPutReader(f, s.File, off, body, 1024, probe, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	rdr, o, final, err := r.GetChunkReader(64, 60)
	if err != nil || !final || o != off {
		t.Fatalf("delegated chunk: o %v final %v err %v", o, final, err)
	}
	b, _ := io.ReadAll(rdr)
	if len(b) < int(body) {
		t.Errorf("delegated chunk %d bytes, want >= %v (the whole window)", len(b), body)
	}
	if _, _, _, err := r.GetChunkReader(64, 60); err != io.EOF {
		t.Errorf("second chunk: want io.EOF, got %v", err)
	}
	if f.ndeleg[0] != 1 {
		t.Errorf("rpcIdx 0 fetched %d times, want exactly 1", f.ndeleg[0])
	}
}

// A delegated reply's buffer points into the shared-memory segment, and the
// segment allocator hands out segment[start:end] — a slice whose capacity runs
// to the end of the segment. The reader must not append onto it: that writes
// over the frames of the replies the cosandbox prefetched for later splits,
// which then fail to unmarshal with valid-looking offsets, far from the cause.
func TestReaderTailDoesNotClobberSegment(t *testing.T) {
	// A stand-in segment: this split's window, followed by the next replies'
	// frames. The delegated buffer is a sub-slice, so its cap covers the rest.
	const winsz = 40
	seg := make([]byte, 4096)
	for i := range seg {
		seg[i] = 'N' // the next replies' bytes
	}
	// A window whose last line has no newline in the probe, forcing extension.
	win := []byte("aaaa bbbb\ncccc dddd eeee ffff gggg hhhh!")
	if len(win) != winsz {
		t.Fatalf("test window is %d bytes, expected %d", len(win), winsz)
	}
	copy(seg[:winsz], win)
	delegated := seg[0:winsz] // exactly what the shmem allocator returns

	data := append(append([]byte{}, win...), []byte(" iiii\njjjj\n")...)
	f := newFakeClnt(data)
	f.delegated[0] = delegated

	s := &mr.Split{File: "name/ux/~local/in", Offset: 0, Length: 20}
	off, body, probe := mr.SplitReadWindow(s, 1024, 4)
	r, err := newGetPutReader(f, s.File, off, body, 1024, probe, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	// The tail must have been extended (the window has no newline after the
	// split end), which is the case that appends.
	rdr, _, _, err := r.GetChunkReader(64, 60)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rdr)
	if len(b) <= winsz {
		t.Fatalf("tail was not extended (%d bytes); the test does not exercise the append", len(b))
	}
	// Everything past the window must be untouched.
	for i := winsz; i < len(seg); i++ {
		if seg[i] != 'N' {
			t.Fatalf("segment clobbered at %d: %q — the reader appended into shared memory, over the next replies' frames", i, seg[i:min(i+16, len(seg))])
		}
	}
}

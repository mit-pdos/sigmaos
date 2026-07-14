package getput

import (
	"bytes"
	"fmt"
	"testing"
)

// fakeClnt is an in-memory clntAPI: gets are served from data, puts are
// recorded. delegated maps rpcIdx to the bytes a cosandbox would have
// prefetched; each idx may be fetched at most once (like the real
// delegated-RPC store, which holds exactly one reply per idx).
type fakeClnt struct {
	data      []byte
	delegated map[uint64][]byte
	ndeleg    map[uint64]int
	ngets     int // direct ranged gets (initial fetch + tail extensions)
	puts      map[uint64][]byte
	objs      map[string][]byte
}

func newFakeClnt(data []byte) *fakeClnt {
	return &fakeClnt{
		data:      data,
		delegated: make(map[uint64][]byte),
		ndeleg:    make(map[uint64]int),
		puts:      make(map[uint64][]byte),
		objs:      make(map[string][]byte),
	}
}

func (f *fakeClnt) getChunk(tgt *Target, off, cnt uint64) ([]byte, error) {
	f.ngets++
	if off >= uint64(len(f.data)) {
		return []byte{}, nil
	}
	end := min(off+cnt, uint64(len(f.data)))
	return f.data[off:end], nil
}

func (f *fakeClnt) delegatedGet(tgt *Target, rpcIdx uint64) ([]byte, error) {
	f.ndeleg[rpcIdx]++
	if f.ndeleg[rpcIdx] > 1 {
		return nil, fmt.Errorf("second delegated get for rpcIdx %d: each idx has exactly one reply", rpcIdx)
	}
	b, ok := f.delegated[rpcIdx]
	if !ok {
		return nil, fmt.Errorf("no delegated reply for rpcIdx %d", rpcIdx)
	}
	return b, nil
}

func (f *fakeClnt) putChunk(tgt *Target, off uint64, b []byte) error {
	f.puts[off] = append([]byte{}, b...)
	return nil
}

func (f *fakeClnt) putObject(tgt *Target, b []byte) error {
	f.objs[tgt.Bucket+"/"+tgt.Key] = append([]byte{}, b...)
	return nil
}

// reassemble the file from recorded putChunk calls
func (f *fakeClnt) reassemble() []byte {
	sz := uint64(0)
	for off, b := range f.puts {
		if end := off + uint64(len(b)); end > sz {
			sz = end
		}
	}
	out := make([]byte, sz)
	for off, b := range f.puts {
		copy(out[off:], b)
	}
	return out
}

func TestClassifyPath(t *testing.T) {
	for _, tc := range []struct {
		pn   string
		want string
		err  bool
	}{
		{"name/s3/~local/9ps3/wiki-2G/f0", "{s3 9ps3 wiki-2G/f0}", false},
		{"name/s3/~any/bkt/key", "{s3 bkt key}", false},
		{"s3clnt/bkt/dir/key", "{s3 bkt dir/key}", false},
		{"name/ux/~local/mr-intermediate/job/shard", "{ux mr-intermediate/job/shard}", false},
		{"name/ux/kid0/f", "{ux f}", false},
		{"name/s3/~local/bktonly", "", true},
		{"name/named/foo", "", true},
	} {
		tgt, err := ClassifyPath(tc.pn)
		if tc.err {
			if err == nil {
				t.Errorf("%q: expected error, got %v", tc.pn, tgt)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", tc.pn, err)
		} else if tgt.String() != tc.want {
			t.Errorf("%q: got %v want %v", tc.pn, tgt, tc.want)
		}
	}
}

func TestWriterUXChunks(t *testing.T) {
	f := newFakeClnt(nil)
	w, err := newGetPutWriter(f, "name/ux/~local/dir/shard")
	if err != nil {
		t.Fatal(err)
	}
	w.chunksz = 10
	var want bytes.Buffer
	for i := range 7 {
		b := bytes.Repeat([]byte{byte('a' + i)}, 3+i)
		want.Write(b)
		if _, err := w.Write(b); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if got := f.reassemble(); !bytes.Equal(got, want.Bytes()) {
		t.Errorf("reassembled %q want %q", got, want.Bytes())
	}
	if int(w.Nbytes()) != want.Len() {
		t.Errorf("Nbytes %v want %v", w.Nbytes(), want.Len())
	}
	if _, ok := f.puts[0]; !ok {
		t.Errorf("no offset-0 chunk (file never created)")
	}
}

func TestWriterUXEmpty(t *testing.T) {
	f := newFakeClnt(nil)
	w, err := newGetPutWriter(f, "name/ux/~local/dir/shard")
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if b, ok := f.puts[0]; !ok || len(b) != 0 {
		t.Errorf("empty shard must still create the file: %v %v", ok, b)
	}
}

func TestWriterS3BufferOnClose(t *testing.T) {
	f := newFakeClnt(nil)
	w, err := newGetPutWriter(f, "name/s3/~local/bkt/dir/out")
	if err != nil {
		t.Fatal(err)
	}
	w.chunksz = 10
	if _, err := w.Write(bytes.Repeat([]byte("x"), 100)); err != nil {
		t.Fatal(err)
	}
	if len(f.objs) != 0 {
		t.Errorf("S3 writer must not put before Close")
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if got := f.objs["bkt/dir/out"]; len(got) != 100 {
		t.Errorf("object len %v want 100", len(got))
	}
}

package getput

import (
	"io"
	"testing"
)

func TestFileReaderDirect(t *testing.T) {
	data := []byte("shard contents, arbitrary bytes\x00\x01\x02")
	f := newFakeClnt(data)
	r, err := newGetPutFileReader(f, "name/ux/kid0/dir/shard", false, 0)
	if err != nil {
		t.Fatalf("newGetPutFileReader: %v", err)
	}
	defer r.Close()
	// One get, to EOF: the reducer consumes the whole shard.
	if f.ngets != 1 {
		t.Errorf("ngets %d want 1", f.ngets)
	}
	if r.Nbytes() != 34 {
		t.Errorf("Nbytes %v want %v", r.Nbytes(), len(data))
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(b) != string(data) {
		t.Errorf("read %q want %q", b, data)
	}
}

func TestFileReaderDelegated(t *testing.T) {
	data := []byte("prefetched by the cosandbox")
	f := newFakeClnt(nil)
	f.delegated[7] = data
	r, err := newGetPutFileReader(f, "name/ux/kid0/dir/shard", true, 7)
	if err != nil {
		t.Fatalf("newGetPutFileReader: %v", err)
	}
	defer r.Close()
	// The delegated reply is the whole fetch: no direct get at all.
	if f.ngets != 0 {
		t.Errorf("ngets %d want 0", f.ngets)
	}
	if r.Nbytes() != 27 {
		t.Errorf("Nbytes %v want %v", r.Nbytes(), len(data))
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(b) != string(data) {
		t.Errorf("read %q want %q", b, data)
	}
}

// A missing prefetch must surface as an error, not a silent empty shard: a
// reducer that read one as empty would drop a mapper's output.
func TestFileReaderDelegatedMissing(t *testing.T) {
	f := newFakeClnt(nil)
	if _, err := newGetPutFileReader(f, "name/ux/kid0/dir/shard", true, 3); err == nil {
		t.Errorf("expected an error for an absent delegated reply")
	}
}

package s3pathclnt

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

// A reader which hands back its data in the given chunk sizes, and can be told
// to deliver the last chunk together with io.EOF — which is what an HTTP
// response body does when the whole body fits in the caller's buffer, and which
// is what fillBuf must not drop.
type chunkedReader struct {
	data      []byte
	chunk     int
	eofWithIt bool // return the final bytes and io.EOF in the same call
	err       error
	off       int
}

func (r *chunkedReader) Read(b []byte) (int, error) {
	if r.off >= len(r.data) {
		if r.err != nil {
			return 0, r.err
		}
		return 0, io.EOF
	}
	n := min(r.chunk, min(len(b), len(r.data)-r.off))
	copy(b, r.data[r.off:r.off+n])
	r.off += n
	if r.off >= len(r.data) {
		if r.err != nil {
			return n, r.err
		}
		if r.eofWithIt {
			return n, io.EOF
		}
	}
	return n, nil
}

func TestFillBuf(t *testing.T) {
	data := []byte("the shard's contents, all 41 bytes of it")
	for _, tc := range []struct {
		name      string
		chunk     int
		eofWithIt bool
		bufsz     int
		want      string
	}{
		// The case that silently lost every small S3 object: one Read returns
		// the whole body and io.EOF together.
		{"whole body with EOF", len(data), true, 1024 * 1024, string(data)},
		// The same body, but EOF arrives in its own call.
		{"whole body then EOF", len(data), false, 1024 * 1024, string(data)},
		// Several chunks, last one carrying EOF.
		{"chunked with EOF", 7, true, 1024, string(data)},
		{"chunked then EOF", 7, false, 1024, string(data)},
		// A buffer smaller than the body: fill it exactly, no EOF involved.
		{"buffer smaller than body", 7, true, 10, string(data[:10])},
	} {
		b := make([]byte, tc.bufsz)
		r := &chunkedReader{data: data, chunk: tc.chunk, eofWithIt: tc.eofWithIt}
		n, err := fillBuf(r, b)
		if err != nil {
			t.Errorf("%s: err %v", tc.name, err)
		}
		if got := string(b[:n]); got != tc.want {
			t.Errorf("%s: read %d bytes %q, want %d %q", tc.name, n, got, len(tc.want), tc.want)
		}
	}
}

// Bytes already in the buffer are reported alongside an error, rather than a
// partial read being turned into nothing.
func TestFillBufPartialThenError(t *testing.T) {
	data := []byte("first part")
	boom := fmt.Errorf("connection reset")
	r := &chunkedReader{data: data, chunk: len(data), err: boom}
	b := make([]byte, 1024)
	n, err := fillBuf(r, b)
	if err != boom {
		t.Errorf("err %v, want %v", err, boom)
	}
	if !bytes.Equal(b[:n], data) {
		t.Errorf("read %q, want %q", b[:n], data)
	}
}

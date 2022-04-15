package fss3

import (
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
)

//
// An implementation of aws's WriteAtBuffer interface that allows
// Read() to retrieve data from the beginning of buf while more data
// is arriving from s3.  Optimistically assumes data arrives in order.
//

type trimBuf struct {
	b     []byte
	nread np.Toffset // the number of bytes trimmed from b
}

func (tb *trimBuf) index(off np.Toffset) np.Toffset {
	return off - tb.nread
}

func (tb *trimBuf) writeAt(p []byte, pos np.Toffset) {
	db.DPrintf("FSS3", "WriteAt %v %v %v\n", len(p), pos, cap(tb.b))
	expLen := np.Tlength(pos) + np.Tlength(len(p))
	if np.Tlength(tb.nread)+np.Tlength(cap(tb.b)) < expLen {
		db.DFatalf("writeAt %v %v\n", pos, len(p))
	}
	if pos < tb.nread {
		// trim p if reader already consumed those bytes and p
		// overlaps with earlier p.
		// https://docs.aws.amazon.com/sdk-for-go/api/aws/#WriteAtBuffer)
		db.DPrintf("FSS3", "trim write o %d cnt %d nread %d\n", pos, len(p), tb.nread)
		n := tb.nread - pos
		if n <= 0 {
			return
		} else {
			p = p[n:]
			pos += n
		}
	}
	copy(tb.b[tb.index(pos):], p)
}

func (tb *trimBuf) read(off np.Toffset, cnt np.Tsize) ([]byte, *np.Err) {
	db.DPrintf("FSS3", "read o %d cnt %d nread %d\n", off, cnt, tb.nread)
	if off < tb.nread {
		np.MkErr(np.TErrInval, off)
	}
	c := np.Toffset(cnt)
	d := tb.b[tb.index(off) : tb.index(off)+c]
	tb.b = tb.b[tb.index(off+c):]
	tb.nread += c
	return d, nil
}

type writeAtBuffer struct {
	sync.Mutex
	c   *sync.Cond
	off np.Toffset // bytes [0, off) are in
	err error
	tb  *trimBuf
}

func mkWriteAtBuffer(sz np.Tlength) *writeAtBuffer {
	b := &writeAtBuffer{}
	b.tb = &trimBuf{}
	b.tb.b = make([]byte, sz)
	b.c = sync.NewCond(&b.Mutex)
	return b
}

func (b *writeAtBuffer) WriteAt(p []byte, pos int64) (n int, err error) {
	b.Lock()
	defer b.Unlock()
	b.tb.writeAt(p, np.Toffset(pos))
	if np.Toffset(pos) == b.off {
		b.off += np.Toffset(len(p))
		b.c.Broadcast()
	}
	return len(p), nil
}

func (b *writeAtBuffer) setErr(err error) {
	b.Lock()
	defer b.Unlock()
	b.err = err
	b.c.Broadcast()
}

// Read data from beginning of buffer.  XXX trim buf (slightly trick
// because WriteAt may over write previously received data.)
func (b *writeAtBuffer) read(off np.Toffset, cnt np.Tsize) ([]byte, *np.Err) {
	b.Lock()
	defer b.Unlock()

	db.DPrintf("FSS3", "Read %d %d\n", off, cnt)
	sz := off + np.Toffset(cnt)
	for b.err == nil && b.off < sz {
		b.c.Wait()
	}
	if b.err != nil {
		return nil, np.MkErr(np.TErrError, b.err)
	}
	return b.tb.read(off, cnt)
}

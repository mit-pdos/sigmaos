package fss3

import (
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
)

//
// An implementation of aws's WriteAtBuffer interface that allows
// Read() to read data from beginning of buf while more data is
// arriving from s3.
//

type writeAtBuffer struct {
	sync.Mutex
	buf []byte
	c   *sync.Cond
	off np.Toffset
	sz  np.Tlength
	err error
}

func mkWriteBuffer(sz np.Tlength) *writeAtBuffer {
	b := &writeAtBuffer{}
	b.buf = make([]byte, sz)
	b.c = sync.NewCond(&b.Mutex)
	return b
}

func (b *writeAtBuffer) WriteAt(p []byte, pos int64) (n int, err error) {
	pLen := np.Tlength(len(p))
	expLen := np.Tlength(pos) + pLen
	b.Lock()
	defer b.Unlock()
	db.DPrintf("FSS3", "WriteAt %v %v\n", len(p), pos)
	if np.Tlength(cap(b.buf)) < expLen {
		db.DFatalf("writeAt %v %v\n", pos, len(p))
	}
	copy(b.buf[pos:], p)
	if b.sz < expLen {
		b.sz = expLen
		b.c.Signal()
	}
	return int(pLen), nil
}

func (b *writeAtBuffer) setErr(err error) {
	b.Lock()
	defer b.Unlock()
	b.err = err
	b.c.Signal()
}

// Read data from beginning of buffer.  XXX trim buf (slightly trick
// because WriteAt may over write previously received data.)
func (b *writeAtBuffer) read(off np.Toffset, cnt np.Tsize) ([]byte, *np.Err) {
	b.Lock()
	defer b.Unlock()

	sz := np.Tlength(off) + np.Tlength(cnt)
	for b.err == nil && b.sz < sz {
		b.c.Wait()
	}
	if b.err != nil {
		return nil, np.MkErr(np.TErrError, b.err)
	}
	return b.buf[off : off+np.Toffset(cnt)], nil
}

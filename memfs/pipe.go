package memfs

import (
	"fmt"
	"sync"
	// "errors"

	db "ulambda/debug"
	np "ulambda/ninep"
	npo "ulambda/npobjsrv"
)

const PIPESZ = 8192

type Pipe struct {
	*Inode
	condr   *sync.Cond
	condw   *sync.Cond
	nreader int
	nwriter int
	buf     []byte
}

func MakePipe(i *Inode) *Pipe {
	pipe := &Pipe{}
	pipe.Inode = i
	pipe.condr = sync.NewCond(&i.mu)
	pipe.condw = sync.NewCond(&i.mu)
	pipe.buf = make([]byte, 0, PIPESZ)
	return pipe
}

func (p *Pipe) Size() np.Tlength {
	return np.Tlength(len(p.buf))
}

func (p *Pipe) SetParent(parent *Dir) {
	p.parent = parent
}

func (p *Pipe) Stat(ctx npo.CtxI) (*np.Stat, error) {
	p.Lock()
	defer p.Unlock()
	st := p.Inode.stat()
	st.Length = np.Tlength(len(p.buf))
	return st, nil
}

func (pipe *Pipe) Open(ctx npo.CtxI, mode np.Tmode) error {
	pipe.Lock()
	defer pipe.Unlock()

	if mode == np.OREAD {
		pipe.nreader += 1
		pipe.condw.Signal()
		for pipe.nwriter == 0 {
			pipe.condr.Wait()
		}
	} else if mode == np.OWRITE {
		pipe.nwriter += 1
		pipe.condr.Signal()
		for pipe.nreader == 0 {
			db.DLPrintf("MEMFS", "Wait for reader\n")
			pipe.condw.Wait()
		}
	} else {
		return fmt.Errorf("Pipe open unknown mode %v\n", mode)
	}
	return nil
}

func (pipe *Pipe) Close(ctx npo.CtxI, mode np.Tmode) error {
	pipe.Lock()
	defer pipe.Unlock()

	if mode == np.OREAD {
		if pipe.nreader < 0 {
			fmt.Errorf("Pipe already closed for reading\n")
		}
		pipe.nreader -= 1
		pipe.condw.Signal()
	} else if mode == np.OWRITE {
		pipe.nwriter -= 1
		if pipe.nwriter < 0 {
			fmt.Errorf("Pipe already closed for writing\n")
		}
		pipe.condr.Signal()
	} else {
		return fmt.Errorf("Pipe open close mode %v\n", mode)
	}
	return nil
}

func (pipe *Pipe) Write(ctx npo.CtxI, d []byte, v np.TQversion) (np.Tsize, error) {
	pipe.Lock()
	defer pipe.Unlock()

	n := len(d)
	for len(d) > 0 {
		for len(pipe.buf) >= PIPESZ {
			if pipe.nreader <= 0 {
				return 0, fmt.Errorf("Pipe write w.o. reader\n")
			}
			pipe.condw.Wait()
		}
		max := len(d)
		if max >= PIPESZ-len(pipe.buf) {
			max = PIPESZ - len(pipe.buf)
		}
		pipe.buf = append(pipe.buf, d[0:max]...)
		d = d[max:]
		pipe.condr.Signal()
	}
	return np.Tsize(n), nil
}

func (pipe *Pipe) Read(ctx npo.CtxI, n np.Tsize, v np.TQversion) ([]byte, error) {
	pipe.Lock()
	defer pipe.Unlock()

	for len(pipe.buf) == 0 {
		if pipe.nwriter <= 0 {
			return nil, nil
		}
		pipe.condr.Wait()
	}
	max := int(n)
	if max >= len(pipe.buf) {
		max = len(pipe.buf)
	}
	d := pipe.buf[0:max]
	pipe.buf = pipe.buf[max:]
	pipe.condw.Signal()
	return d, nil
}

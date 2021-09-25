package memfs

import (
	"fmt"
	"sync"
	// "errors"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
)

const PIPESZ = 8192

type Pipe struct {
	fs.FsObj
	condr   *sync.Cond
	condw   *sync.Cond
	nreader int
	nwriter int
	buf     []byte
}

func MakePipe(i fs.FsObj) *Pipe {
	pipe := &Pipe{}
	pipe.FsObj = i
	pipe.condr = sync.NewCond(i.LockAddr())
	pipe.condw = sync.NewCond(i.LockAddr())
	pipe.buf = make([]byte, 0, PIPESZ)
	return pipe
}

func (p *Pipe) Size() np.Tlength {
	return np.Tlength(len(p.buf))
}

func (p *Pipe) Stat(ctx fs.CtxI) (*np.Stat, error) {
	p.Lock()
	defer p.Unlock()
	st, err := p.FsObj.Stat(ctx)
	if err != nil {
		return nil, err
	}
	st.Length = np.Tlength(len(p.buf))
	return st, nil
}

func (pipe *Pipe) Open(ctx fs.CtxI, mode np.Tmode) (fs.FsObj, error) {
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
		return nil, fmt.Errorf("Pipe open unknown mode %v\n", mode)
	}
	return nil, nil
}

func (pipe *Pipe) Close(ctx fs.CtxI, mode np.Tmode) error {
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

func (pipe *Pipe) Write(ctx fs.CtxI, o np.Toffset, d []byte, v np.TQversion) (np.Tsize, error) {
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

func (pipe *Pipe) Read(ctx fs.CtxI, o np.Toffset, n np.Tsize, v np.TQversion) ([]byte, error) {
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

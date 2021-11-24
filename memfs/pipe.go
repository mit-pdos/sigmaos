package memfs

import (
	"fmt"
	"io"
	"log"
	"sync"
	// "errors"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
)

const PIPESZ = 8192

type Pipe struct {
	fs.FsObj
	mu      sync.Mutex
	condr   *sync.Cond
	condw   *sync.Cond
	nreader int
	nwriter int
	buf     []byte
	nlink   int
}

func MakePipe(i fs.FsObj) *Pipe {
	pipe := &Pipe{}
	pipe.FsObj = i
	pipe.condr = sync.NewCond(&pipe.mu)
	pipe.condw = sync.NewCond(&pipe.mu)
	pipe.buf = make([]byte, 0, PIPESZ)
	pipe.nlink = 1
	pipe.nreader = 0
	pipe.nwriter = 0
	return pipe
}

func (p *Pipe) Size() np.Tlength {
	return np.Tlength(len(p.buf))
}

func (p *Pipe) Stat(ctx fs.CtxI) (*np.Stat, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	st, err := p.FsObj.Stat(ctx)
	if err != nil {
		return nil, err
	}
	st.Length = np.Tlength(len(p.buf))
	return st, nil
}

func (pipe *Pipe) Open(ctx fs.CtxI, mode np.Tmode) (fs.FsObj, error) {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	log.Printf("open %p m %v r %v w %v\n", pipe, mode, pipe.nreader, pipe.nwriter)

	if mode == np.OREAD {
		pipe.nreader += 1
		pipe.condw.Signal()
		for pipe.nwriter == 0 {
			pipe.condr.Wait()
			if pipe.nlink == 0 {
				return nil, fmt.Errorf("Pipe removed")
			}
		}
	} else if mode == np.OWRITE {
		pipe.nwriter += 1
		pipe.condr.Signal()
		for pipe.nreader == 0 {
			db.DLPrintf("MEMFS", "Wait for reader\n")
			pipe.condw.Wait()
			if pipe.nlink == 0 {
				return nil, fmt.Errorf("Pipe removed")
			}

		}
	} else {
		return nil, fmt.Errorf("Pipe open unknown mode %v\n", mode)
	}
	return nil, nil
}

func (pipe *Pipe) Close(ctx fs.CtxI, mode np.Tmode) error {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	log.Printf("%p: pipeclose %v\n", pipe, mode)
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
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	n := len(d)
	for len(d) > 0 {
		for len(pipe.buf) >= PIPESZ {
			if pipe.nreader <= 0 {
				return 0, io.EOF
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
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	for len(pipe.buf) == 0 {
		if pipe.nwriter <= 0 {
			return nil, io.EOF
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

func (pipe *Pipe) Unlink(ctx fs.CtxI) error {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	pipe.nlink -= 1
	pipe.condw.Signal()
	pipe.condr.Signal()
	return nil
}

package memfs

import (
	"fmt"
	"log"
	"sync"
	// "errors"

	//	"github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/sesscond"
)

const PIPESZ = 8192

type Pipe struct {
	fs.Inode
	mu sync.Mutex
	//mu      deadlock.Mutex
	condr   *sesscond.SessCond
	condw   *sesscond.SessCond
	nreader int
	nwriter int
	wclosed bool
	rclosed bool
	buf     []byte
}

func MakePipe(ctx fs.CtxI, i fs.Inode) *Pipe {
	pipe := &Pipe{}
	pipe.Inode = i
	pipe.condr = ctx.SessCondTable().MakeSessCond(&pipe.mu)
	pipe.condw = ctx.SessCondTable().MakeSessCond(&pipe.mu)
	pipe.buf = make([]byte, 0, PIPESZ)
	pipe.nreader = 0
	pipe.nwriter = 0
	pipe.wclosed = false
	pipe.rclosed = false
	return pipe
}

func (p *Pipe) Size() np.Tlength {
	return np.Tlength(len(p.buf))
}

func (p *Pipe) Stat(ctx fs.CtxI) (*np.Stat, *np.Err) {
	p.mu.Lock()
	defer p.mu.Unlock()
	st, err := p.Inode.Stat(ctx)
	if err != nil {
		return nil, err
	}
	st.Length = np.Tlength(len(p.buf))
	return st, nil
}

func (pipe *Pipe) Open(ctx fs.CtxI, mode np.Tmode) (fs.FsObj, *np.Err) {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	if mode == np.OREAD {
		if pipe.rclosed || pipe.Nlink() <= 0 {
			return nil, np.MkErr(np.TErrClosed, "pipe reading")
		}
		pipe.nreader += 1
		//log.Printf("%v/%v: open pipe %p for reading %v\n", ctx.Uname(), ctx.SessionId(), pipe, pipe.nreader)
		pipe.condw.Signal()
		for pipe.nwriter == 0 && !pipe.wclosed {
			err := pipe.condr.Wait(ctx.SessionId())
			if err != nil {
				pipe.nreader -= 1
				if pipe.nreader == 0 {
					pipe.rclosed = true
				}
				return nil, err
			}
			if pipe.Nlink() == 0 {
				return nil, np.MkErr(np.TErrNotfound, "pipe")
			}
		}
	} else if mode == np.OWRITE {
		if pipe.wclosed || pipe.Nlink() <= 0 {
			return nil, np.MkErr(np.TErrClosed, "pipe writing")
		}
		pipe.nwriter += 1
		// log.Printf("%v/%v: open pipe %p for writing %v\n", ctx.Uname(), ctx.SessionId(), pipe, pipe.nwriter)
		pipe.condr.Signal()
		for pipe.nreader == 0 && !pipe.rclosed {
			db.DLPrintf("MEMFS", "Wait for reader\n")
			err := pipe.condw.Wait(ctx.SessionId())
			if err != nil {
				pipe.nwriter -= 1
				if pipe.nwriter == 0 {
					pipe.wclosed = true
				}
				return nil, err
			}
			if pipe.Nlink() == 0 {
				return nil, np.MkErr(np.TErrNotfound, "pipe")
			}

		}
	} else {
		return nil, np.MkErr(np.TErrInval, fmt.Sprintf("mode %v", mode))
	}
	return nil, nil
}

func (pipe *Pipe) Close(ctx fs.CtxI, mode np.Tmode) *np.Err {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	//log.Printf("%v: close %v pipe %v\n", ctx.Uname(), mode, pipe.nwriter)
	if mode == np.OREAD {
		pipe.nreader -= 1
		if pipe.nreader == 0 {
			pipe.rclosed = true
		}
		if pipe.nreader < 0 {
			np.MkErr(np.TErrClosed, "pipe reading")
		}
		pipe.condw.Signal()
	} else if mode == np.OWRITE {
		pipe.nwriter -= 1
		if pipe.nwriter == 0 {
			pipe.wclosed = true
		}
		if pipe.nwriter < 0 {
			np.MkErr(np.TErrClosed, "pipe writing")
		}
		pipe.condr.Signal()
	} else {
		return np.MkErr(np.TErrInval, fmt.Sprintf("mode %v", mode))
	}
	return nil
}

func (pipe *Pipe) Write(ctx fs.CtxI, o np.Toffset, d []byte, v np.TQversion) (np.Tsize, *np.Err) {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	n := len(d)
	for len(d) > 0 {
		for len(pipe.buf) >= PIPESZ {
			if pipe.nreader <= 0 {
				return 0, np.MkErr(np.TErrClosed, "pipe")
			}
			err := pipe.condw.Wait(ctx.SessionId())
			if err != nil {
				return 0, err
			}
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

func (pipe *Pipe) Read(ctx fs.CtxI, o np.Toffset, n np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	for len(pipe.buf) == 0 {
		if pipe.nwriter <= 0 {
			return nil, np.MkErr(np.TErrClosed, "pipe")
		}
		err := pipe.condr.Wait(ctx.SessionId())
		if err != nil {
			return nil, err
		}
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

func (pipe *Pipe) Unlink() {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	pipe.DecNlink()
	pipe.condw.Signal()
	pipe.condr.Signal()
}

func (pipe *Pipe) Snapshot(fn fs.SnapshotF) []byte {
	log.Fatalf("FATAL tried to snapshot pipe")
	return nil
}

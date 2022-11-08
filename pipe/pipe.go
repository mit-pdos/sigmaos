package pipe

import (
	"fmt"
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	"sigmaos/fs"
	np "sigmaos/ninep"
	"sigmaos/sesscond"
)

const PIPESZ = 8192

type Pipe struct {
	mu      sync.Mutex
	condr   *sesscond.SessCond
	condw   *sesscond.SessCond
	sct     *sesscond.SessCondTable
	nreader int
	nwriter int
	wclosed bool
	rclosed bool
	nlink   int
	buf     []byte
}

func MakePipe(ctx fs.CtxI) *Pipe {
	pipe := &Pipe{}
	pipe.condr = ctx.SessCondTable().MakeSessCond(&pipe.mu)
	pipe.condw = ctx.SessCondTable().MakeSessCond(&pipe.mu)
	pipe.sct = ctx.SessCondTable()
	pipe.buf = make([]byte, 0, PIPESZ)
	pipe.nreader = 0
	pipe.nwriter = 0
	pipe.wclosed = false
	pipe.rclosed = false
	pipe.nlink = 1
	return pipe
}

func (pipe *Pipe) Open(ctx fs.CtxI, mode np.Tmode) (fs.FsObj, *np.Err) {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	if mode == np.OREAD {
		if pipe.rclosed || pipe.nlink <= 0 {
			return nil, np.MkErr(np.TErrClosed, "pipe reading")
		}
		pipe.nreader += 1
		db.DPrintf("PIPE", "%v/%v: open pipe %p for reading %v\n", ctx.Uname(), ctx.SessionId(), pipe, pipe.nreader)
		pipe.condw.Signal()
		for pipe.nwriter == 0 && !pipe.wclosed {
			db.DPrintf("PIPE", "Wait for writer %v\n", ctx.SessionId())
			err := pipe.condr.Wait(ctx.SessionId())
			if err != nil {
				pipe.nreader -= 1
				if pipe.nreader == 0 {
					pipe.rclosed = true
				}
				return nil, err
			}
			if pipe.nlink == 0 {
				return nil, np.MkErr(np.TErrNotfound, "pipe")
			}
		}
	} else if mode == np.OWRITE {
		if pipe.wclosed || pipe.nlink <= 0 {
			return nil, np.MkErr(np.TErrClosed, "pipe writing")
		}
		pipe.nwriter += 1
		db.DPrintf("PIPE", "%v/%v: open pipe %p for writing %v\n", ctx.Uname(), ctx.SessionId(), pipe, pipe.nwriter)
		pipe.condr.Signal()
		for pipe.nreader == 0 && !pipe.rclosed {
			db.DPrintf("PIPE", "Wait for reader %v\n", ctx.SessionId())
			err := pipe.condw.Wait(ctx.SessionId())
			if err != nil {
				db.DPrintf("PIPE", "Wait reader err %v %v\n", err, ctx.SessionId())
				pipe.nwriter -= 1
				if pipe.nwriter == 0 {
					pipe.wclosed = true
				}
				return nil, err
			}
			if pipe.nlink == 0 {
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

	db.DPrintf("PIPE", "%v: close %v pipe %v\n", ctx.Uname(), mode, pipe.nwriter)
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

	db.DPrintf("PIPE", "Unlink: %v\n", pipe)

	pipe.nlink -= 1
	pipe.condw.Signal()
	pipe.condr.Signal()

	// Free sess conds.
	if pipe.nlink == 0 {
		pipe.sct.FreeSessCond(pipe.condw)
		pipe.sct.FreeSessCond(pipe.condr)
	}
}

func (pipe *Pipe) Size() np.Tlength {
	return np.Tlength(len(pipe.buf))
}

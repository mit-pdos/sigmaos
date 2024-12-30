package pipe

import (
	"fmt"
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	"sigmaos/sigmasrv/clntcond"
	db "sigmaos/debug"
	"sigmaos/api/fs"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

const PIPESZ = 8192

type Pipe struct {
	mu      sync.Mutex
	condr   *clntcond.ClntCond
	condw   *clntcond.ClntCond
	sct     *clntcond.ClntCondTable
	nreader int
	nwriter int
	wclosed bool
	rclosed bool
	nlink   int
	buf     []byte
}

func NewPipe(ctx fs.CtxI) *Pipe {
	pipe := &Pipe{}
	pipe.condr = ctx.ClntCondTable().NewClntCond(&pipe.mu)
	pipe.condw = ctx.ClntCondTable().NewClntCond(&pipe.mu)
	pipe.sct = ctx.ClntCondTable()
	pipe.buf = make([]byte, 0, PIPESZ)
	pipe.nreader = 0
	pipe.nwriter = 0
	pipe.wclosed = false
	pipe.rclosed = false
	pipe.nlink = 1
	return pipe
}

func (pipe *Pipe) Open(ctx fs.CtxI, mode sp.Tmode) (fs.FsObj, *serr.Err) {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	if mode == sp.OREAD {
		if pipe.rclosed || pipe.nlink <= 0 {
			return nil, serr.NewErr(serr.TErrClosed, "pipe reading")
		}
		pipe.nreader += 1
		db.DPrintf(db.PIPE, "%v/%v: open pipe %v(%p) for reading %v\n", ctx.Principal(), ctx.SessionId(), pipe, pipe, pipe.nreader)
		pipe.condw.Signal()
		for pipe.nwriter == 0 && !pipe.wclosed {
			db.DPrintf(db.PIPE, "Wait for writer %v\n", ctx.SessionId())
			err := pipe.condr.Wait(ctx.ClntId())
			if err != nil {
				pipe.nreader -= 1
				if pipe.nreader == 0 {
					pipe.rclosed = true
				}
				return nil, err
			}
			if pipe.nlink == 0 {
				return nil, serr.NewErr(serr.TErrNotfound, "pipe")
			}
			db.DPrintf(db.PIPE, "%v/%v Open pipe %v(%p) for reader\n", ctx.Principal(), ctx.SessionId(), pipe, pipe)
		}
	} else if mode == sp.OWRITE {
		if pipe.wclosed || pipe.nlink <= 0 {
			return nil, serr.NewErr(serr.TErrClosed, "pipe writing")
		}
		pipe.nwriter += 1
		db.DPrintf(db.PIPE, "%v/%v: open pipe %v(%p) for writing %v\n", ctx.Principal(), ctx.SessionId(), pipe, pipe, pipe.nwriter)
		pipe.condr.Signal()
		for pipe.nreader == 0 && !pipe.rclosed {
			db.DPrintf(db.PIPE, "Wait for reader %v\n", ctx.SessionId())
			err := pipe.condw.Wait(ctx.ClntId())
			if err != nil {
				db.DPrintf(db.PIPE, "Wait reader err %v %v\n", err, ctx.SessionId())
				pipe.nwriter -= 1
				if pipe.nwriter == 0 {
					pipe.wclosed = true
				}
				return nil, err
			}
			if pipe.nlink == 0 {
				return nil, serr.NewErr(serr.TErrNotfound, "pipe")
			}
			db.DPrintf(db.PIPE, "%v/%v Open pipe %v(%p) for writer\n", ctx.Principal(), ctx.SessionId(), pipe, pipe)
		}
	} else {
		return nil, serr.NewErr(serr.TErrInval, fmt.Sprintf("mode %v", mode))
	}
	return nil, nil
}

func (pipe *Pipe) Close(ctx fs.CtxI, mode sp.Tmode) *serr.Err {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	db.DPrintf(db.PIPE, "%v: close %v pipe %v\n", ctx.Principal(), mode, pipe.nwriter)
	if mode == sp.OREAD {
		pipe.nreader -= 1
		if pipe.nreader == 0 {
			pipe.rclosed = true
		}
		if pipe.nreader < 0 {
			serr.NewErr(serr.TErrClosed, "pipe reading")
		}
		pipe.condw.Signal()
	} else if mode == sp.OWRITE {
		pipe.nwriter -= 1
		if pipe.nwriter == 0 {
			pipe.wclosed = true
		}
		if pipe.nwriter < 0 {
			serr.NewErr(serr.TErrClosed, "pipe writing")
		}
		pipe.condr.Signal()
	} else {
		return serr.NewErr(serr.TErrInval, fmt.Sprintf("mode %v", mode))
	}
	return nil
}

func (pipe *Pipe) Write(ctx fs.CtxI, o sp.Toffset, d []byte, f sp.Tfence) (sp.Tsize, *serr.Err) {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	db.DPrintf(db.PIPE, "%v/%v: Write pipe %d %v(%p)\n", ctx.Principal(), ctx.SessionId(), len(d), pipe, pipe)

	n := len(d)
	for len(d) > 0 {
		for len(pipe.buf) >= PIPESZ {
			if pipe.nreader <= 0 {
				return 0, serr.NewErr(serr.TErrClosed, "pipe")
			}
			db.DPrintf(db.PIPE, "%v/%v: Write wait for reader %v(%p)\n", ctx.Principal(), ctx.SessionId(), pipe, pipe)
			err := pipe.condw.Wait(ctx.ClntId())
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
	return sp.Tsize(n), nil
}

func (pipe *Pipe) Read(ctx fs.CtxI, o sp.Toffset, n sp.Tsize, f sp.Tfence) ([]byte, *serr.Err) {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	db.DPrintf(db.PIPE, "%v/%v: Read pipe %v(%p)\n", ctx.Principal(), ctx.SessionId(), pipe, pipe)

	for len(pipe.buf) == 0 {
		if pipe.nwriter <= 0 {
			return nil, serr.NewErr(serr.TErrClosed, "pipe")
		}
		db.DPrintf(db.PIPE, "%v/%v: Read wait for writer %v(%p)\n", ctx.Principal(), ctx.SessionId(), pipe, pipe)
		err := pipe.condr.Wait(ctx.ClntId())
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

	db.DPrintf(db.PIPE, "Unlink: %v\n", pipe)

	pipe.nlink -= 1
	pipe.condw.Signal()
	pipe.condr.Signal()

	// Free sess conds.
	if pipe.nlink == 0 {
		db.DPrintf(db.PIPE, "PIPE NLINK 0")
		pipe.sct.FreeClntCond(pipe.condw)
		pipe.sct.FreeClntCond(pipe.condr)
	}
}

func (pipe *Pipe) Size() sp.Tlength {
	return sp.Tlength(len(pipe.buf))
}

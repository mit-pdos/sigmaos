package memfs

import (
	"fmt"
	"sync"
	// "errors"

	np "ulambda/ninep"
)

const PIPESZ = 8192

type Pipe struct {
	mu      sync.Mutex
	condr   *sync.Cond
	condw   *sync.Cond
	nreader int
	nwriter int
	buf     []byte
}

func MakePipe() *Pipe {
	pipe := &Pipe{}
	pipe.condr = sync.NewCond(&pipe.mu)
	pipe.condw = sync.NewCond(&pipe.mu)
	pipe.buf = make([]byte, 0, PIPESZ)
	return pipe
}

func (p *Pipe) Len() np.Tlength {
	return np.Tlength(len(p.buf))
}

func (pipe *Pipe) open(mode np.Tmode) error {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

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
			pipe.condw.Wait()
		}
	} else {
		return fmt.Errorf("Pipe open unknown mode %v\n", mode)
	}
	return nil
}

func (pipe *Pipe) close(mode np.Tmode) error {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

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

func (pipe *Pipe) write(d []byte) (np.Tsize, error) {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

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

func (pipe *Pipe) read(n np.Tsize) ([]byte, error) {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

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

package memfs

import (
	"sync"
	// "errors"

	np "ulambda/ninep"
)

const PIPESZ = 8192

type Pipe struct {
	mu    sync.Mutex
	condr *sync.Cond
	condw *sync.Cond
	buf   []byte
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

func (pipe *Pipe) write(d []byte) (np.Tsize, error) {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	n := len(d)
	for len(d) > 0 {
		for len(pipe.buf) >= PIPESZ {
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

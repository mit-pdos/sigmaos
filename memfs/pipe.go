package memfs

import (
	"sync"
	// "errors"

	np "ulambda/ninep"
)

type Pipe struct {
	mu   sync.Mutex
	cond *sync.Cond
	buf  []byte
}

func MakePipe() *Pipe {
	pipe := &Pipe{}
	pipe.cond = sync.NewCond(&pipe.mu)
	pipe.buf = make([]byte, 0, 1024)
	return pipe
}

func (p *Pipe) Len() np.Tlength {
	return np.Tlength(len(p.buf))
}

// XXX if full block writer
func (pipe *Pipe) Write(d []byte) (np.Tsize, error) {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	pipe.buf = append(pipe.buf, d...)
	pipe.cond.Signal()
	return np.Tsize(len(d)), nil
}

// XXX read no more than n
func (pipe *Pipe) Read(n np.Tsize) ([]byte, error) {
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	for len(pipe.buf) == 0 {
		pipe.cond.Wait()
	}
	d := pipe.buf
	pipe.buf = make([]byte, 0, 1024)
	return d, nil
}

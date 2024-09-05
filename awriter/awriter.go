package awriter

import (
	"fmt"
	"io"
	"sync"

	db "sigmaos/debug"
	// sp "sigmaos/sigmap"
)

type Writer struct {
	sync.Mutex
	producer  *sync.Cond
	consumer  *sync.Cond
	wrt       io.Writer
	buffs     [][]byte
	lens      []int
	fullIdxs  []int
	emptyIdxs []int
	exit      bool
	err       error
}

func NewWriterSize(wrt io.Writer, nbuf, sz int) *Writer {
	w := &Writer{}
	w.wrt = wrt
	w.producer = sync.NewCond(&w.Mutex)
	w.consumer = sync.NewCond(&w.Mutex)
	w.buffs = make([][]byte, 0, nbuf)
	w.lens = make([]int, 0, nbuf)
	w.fullIdxs = make([]int, 0, nbuf)
	w.emptyIdxs = make([]int, 0, nbuf)
	for i := 0; i < nbuf; i++ {
		w.buffs = append(w.buffs, make([]byte, sz))
		w.lens = append(w.lens, 0)
		w.emptyIdxs = append(w.emptyIdxs, i)
	}
	go w.writer()
	return w
}

func (w *Writer) writer() {
	w.Lock()
	defer w.Unlock()

	for !w.exit {
		for len(w.fullIdxs) == 0 && !w.exit {
			w.consumer.Wait()
		}
		if w.exit {
			break
		}
		// Remove the buff from the list of full buffs.
		var idx int
		idx, w.fullIdxs = w.fullIdxs[0], w.fullIdxs[1:]
		m := w.lens[idx]
		d := w.buffs[idx][0:m]
		db.DPrintf(db.AWRITER, "%p writer %v", w.wrt, m)
		w.Unlock()

		// write without holding lock
		n, err := w.wrt.Write(d)

		w.Lock()
		if err != nil {
			w.err = err
		} else if n != m {
			w.err = io.ErrShortWrite
		}
		w.lens[idx] = 0
		// Append to the list of empty buffer indices
		w.emptyIdxs = append(w.emptyIdxs, idx)
		w.producer.Broadcast()
	}
	db.DPrintf(db.AWRITER, "%p writer exit", w.wrt)
}

func (w *Writer) Write(p []byte) (int, error) {
	w.Lock()
	defer w.Unlock()

	db.DPrintf(db.AWRITER, "awrite %p lens %v empty %v full %v wlen %v", w.wrt, w.lens, w.emptyIdxs, w.fullIdxs, len(p))

	if w.exit {
		return 0, fmt.Errorf("Writer is closed")
	}

	for len(w.emptyIdxs) == 0 && w.err == nil {
		w.producer.Wait()
	}
	if w.err != nil {
		return 0, w.err
	}
	// Get the index of the next empty buffer
	var idx int
	idx, w.emptyIdxs = w.emptyIdxs[0], w.emptyIdxs[1:]

	// Release the lock while doing the copy
	w.Unlock()
	copy(w.buffs[idx], p)
	w.lens[idx] = len(p)

	// Grab the lock again
	w.Lock()
	// Add the buffer index to the list of full buffer indices
	w.fullIdxs = append(w.fullIdxs, idx)
	w.consumer.Signal()
	return len(p), nil
}

func (w *Writer) Close() error {
	w.Lock()
	defer w.Unlock()

	db.DPrintf(db.AWRITER, "close awrite %p %v", w.wrt, w.exit)
	if w.exit {
		return fmt.Errorf("Writer is closed")
	}

	for len(w.emptyIdxs) < len(w.buffs) && w.err == nil {
		w.producer.Wait()
	}
	if w.err != nil {
		db.DPrintf(db.ALWAYS, "Err when closing: %v len %v", w.err, len(w.emptyIdxs))
	}
	w.exit = true
	w.consumer.Signal()
	return w.err
}

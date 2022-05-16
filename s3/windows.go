package fss3

import (
	"fmt"
	// "log"

	np "ulambda/ninep"
)

type window struct {
	start np.Toffset
	end   np.Toffset
}

func (w *window) String() string {
	return fmt.Sprintf("[%d, %d)", w.start, w.end)
}

type windows struct {
	ws []*window
}

func mkWindows() *windows {
	return &windows{make([]*window, 0)}
}

// maybe merge with wi with wi+1
func (ws *windows) merge(i int) {
	w := ws.ws[i]
	if len(ws.ws) > i+1 { // is there a next w?
		w1 := ws.ws[i+1]
		if w.end >= w1.start { // merge w1 into w
			if w1.end > w.end {
				w.end = w1.end
			}
			if i+2 == len(ws.ws) { // trim i+1
				ws.ws = ws.ws[0 : i+1]
			} else {
				ws.ws = append(ws.ws[0:i+1], ws.ws[i+2:]...)
			}
		}
	}
}

func (ws *windows) insertw(n *window) {
	for i, w := range ws.ws {
		if n.start > w.end { // n is beyond w
			continue
		}
		if n.end < w.start { // n preceeds w
			ws.ws = append(ws.ws[:i+1], ws.ws[i:]...)
			ws.ws[i] = n
			return
		}
		// n overlaps w
		if n.start < w.start {
			w.start = n.start
		}
		if n.end > w.end {
			w.end = n.end
			ws.merge(i)
			return
		}
		return
	}
	ws.ws = append(ws.ws, n)
}

// Caller has received [0, off). new data for [start, end).  Returns
// the number of bytes to increase offset with.
func (ws *windows) add(off, start, end np.Toffset) np.Toffset {
	ws.insertw(&window{start, end})
	w0 := ws.ws[0]
	if w0.start > off { // out of order
		return 0
	}
	if w0.start < off { // new data may have straggle off
		w0.start = off
	}
	ws.ws = ws.ws[1:]
	return w0.end - w0.start
}

package threadmgr

import (
	"log"
	"sync"

	np "ulambda/ninep"
)

const (
	MAX_NWAKE = 1000 // Max number of wakes in a single operation.
)

type Thread struct {
	*sync.Mutex
	done   bool
	opC    chan *Op
	sleepC chan bool
	wakeC  chan *sync.Cond
	nwake  int
	pfn    ProcessFn
}

func makeThread(pfn ProcessFn) *Thread {
	t := &Thread{}
	t.Mutex = &sync.Mutex{}
	t.opC = make(chan *Op)
	t.sleepC = make(chan bool)
	t.wakeC = make(chan *sync.Cond, MAX_NWAKE)
	t.pfn = pfn
	return t
}

func (t *Thread) Process(fc *np.Fcall, replies chan *np.Fcall) {
	t.opC <- makeOp(fc, replies)
}

// Called when an operation is going to sleep (or has terminated). If the op
// has terminated, c will be nil.  This function assumes that there is only
// ever one goroutine waiting on each cond.
func (t *Thread) Sleep(c *sync.Cond) {
	t.sleepC <- true
	if c != nil {
		c.Wait()
	}
}

// Called when an operation is going to be woken up. This function
// assumes that there is only ever one goroutine waiting on each cond.
func (t *Thread) Wake(c *sync.Cond) {
	t.nwake += 1
	if t.nwake >= MAX_NWAKE {
		log.Fatalf("ThreadMgr: Too many wakeups in a single op")
	}
	t.wakeC <- c
}

// Wait until an op is either terminated or sleeping.
func (t *Thread) wait() {
	<-t.sleepC
}

// Processes operations on a single channel.
func (t *Thread) processOps() {
	for !t.isDone() {
		op := <-t.opC
		// Process the operation.
		go func() {
			t.pfn(op.fc, op.replies)
			t.Sleep(nil)
		}()
		// Wait for it to sleep or complete.
		t.wait()
		// Process any wakeups it may have generated.
		t.processWakeups()
	}
}

// Processes wakeups on a single channel.
func (t *Thread) processWakeups() {
	for i := 0; i < t.nwake; i++ {
		c := <-t.wakeC
		// Actually wake up the operation.
		c.Signal()
		// Wait for the operation to sleep or terminate.
		t.wait()
	}
	t.nwake = 0
}

func (t *Thread) start() {
	go t.processOps()
}

func (t *Thread) stop() {
	t.Lock()
	defer t.Unlock()
	t.done = true
}

func (t *Thread) isDone() bool {
	t.Lock()
	defer t.Unlock()
	return t.done
}

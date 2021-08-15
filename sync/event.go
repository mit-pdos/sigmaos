package sync

import (
	"ulambda/fslib"
)

// An asynchronous event.
type Event struct {
	path string // Path to the event
	c    *Cond  // Non-exclusive condition variable to wait on.
	*fslib.FsLib
}

func MakeEvent(fsl *fslib.FsLib, path string) *Event {
	e := &Event{}
	e.path = path
	e.c = MakeCond(fsl, path, nil)
	e.FsLib = fsl
	return e
}

func (e *Event) Init() {
	e.c.Init()
}

// Wait for the event to be triggered...
func (e *Event) Wait() {
	e.c.Wait()
}

// Wake up all waiters...
func (e *Event) Broadcast() {
	e.c.Broadcast()
}

func (e *Event) Destroy() []string {
	return e.c.Destroy()
}

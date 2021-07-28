package sync

import (
	"log"
	"path"
)

const (
	COND_LOCK = "COND_LOCK"
)

type Cond struct {
	path     string
	lockPath string
}

func MakeCond(cpath string) *Cond {
	c := &Cond{}
	c.path = cpath
	c.lockPath = path.Join(cpath, COND_LOCK)

	err := fsl.Mkdir(cpath, 0777)
	if err != nil {
		log.Fatalf("Error creating cond variable: %v", err)
	}
	return c
}

// Wait.
func (c *Cond) Wait() {
	fsl.LockFile(c.path, COND_LOCK)
	defer fsl.UnlockFile(c.path, COND_LOCK)

}

// Wake up all waiters.
func (c *Cond) Broadcast() {
	fsl.LockFile(c.path, COND_LOCK)
	defer fsl.UnlockFile(c.path, COND_LOCK)

}

// Wake up one waiter.
func (c *Cond) Signal() {
	fsl.LockFile(c.path, COND_LOCK)
	defer fsl.UnlockFile(c.path, COND_LOCK)

}

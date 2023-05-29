package container

import (
	"io"
	"os"
	"path"
	"strconv"
	"strings"

	db "sigmaos/debug"
)

func (c *Container) getCPUShares() int64 {
	p := path.Join(c.cgroupPath, "cpu.weight")
	f, err := os.Open(p)
	if err != nil {
		db.DFatalf("Error open: %v", err)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		db.DFatalf("Error read: %v", err)
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		db.DFatalf("Error strconv: %v", err)
	}
	err = f.Close()
	if err != nil {
		db.DFatalf("Error close: %v", err)
	}
	return n
}

func (c *Container) setCPUShares(n int64) {
	p := path.Join(c.cgroupPath, "cpu.weight")
	f, err := os.OpenFile(p, os.O_RDWR, 0)
	if err != nil {
		db.DFatalf("Error open: %v", err)
	}
	b := []byte(strconv.FormatInt(n, 10))
	n2, err := f.Write(b)
	if err != nil || n2 != len(b) {
		db.DPrintf(db.ALWAYS, "Error write: %v n: %v", err, n2)
		return
	}
	err = f.Close()
	if err != nil {
		db.DFatalf("Error close: %v", err)
	}
}

func (c *Container) setMemoryLimit(membytes int64, memswap int64) {
	ps := []string{
		path.Join(c.cgroupPath, "memory.max"),
		path.Join(c.cgroupPath, "memory.swap.max"),
	}
	vals := []int64{
		membytes,
		memswap,
	}
	for i := range ps {
		p := ps[i]
		m := vals[i]
		f, err := os.OpenFile(p, os.O_RDWR, 0)
		if err != nil {
			db.DFatalf("Error open: %v", err)
		}
		b := []byte(strconv.FormatInt(m, 10))
		n, err := f.Write(b)
		if err != nil || n != len(b) {
			db.DPrintf(db.ALWAYS, "Error write: %v n: %v", err, n)
			return
		}
		err = f.Close()
		if err != nil {
			db.DFatalf("Error close: %v", err)
		}
	}
}

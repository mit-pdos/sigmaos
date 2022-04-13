package fss3

import (
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
)

type Cache struct {
	sync.Mutex
	cache map[string]*info
}

func mkCache() *Cache {
	c := &Cache{}
	c.cache = make(map[string]*info)
	return c
}

func (c *Cache) lookup(path np.Path) *info {
	c.Lock()
	defer c.Unlock()
	if o, ok := c.cache[path.String()]; ok {
		db.DPrintf("FSS3", "cache hit %v \n", path)
		return o
	}
	return nil
}

func (c *Cache) insert(path np.Path, i *info) bool {
	c.Lock()
	defer c.Unlock()
	s := path.String()
	if _, ok := c.cache[s]; ok {
		return false
	}
	c.cache[s] = i
	return true
}

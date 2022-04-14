package fss3

import (
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
)

// XXX evict entries

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
		return o
	}
	return nil
}

func (c *Cache) insert(path np.Path, i *info) {
	c.Lock()
	defer c.Unlock()
	db.DPrintf("FSS3", "path %v insert %v\n", path, i)
	s := path.String()
	c.cache[s] = i
}

func (c *Cache) delete(path np.Path) bool {
	c.Lock()
	defer c.Unlock()
	db.DPrintf("FSS3", "cache: delete %v\n", path)
	s := path.String()
	if _, ok := c.cache[s]; ok {
		return false
	}
	delete(c.cache, s)
	return true
}

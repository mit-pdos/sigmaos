package mr

import (
	"github.com/fmstephe/unsafeutil"
)

type kvmap struct {
	mincap int
	kvs    map[string]*values
}

type values struct {
	k  string
	vs []string
}

func newKvmap(mincap int) *kvmap {
	return &kvmap{
		mincap: mincap,
		kvs:    make(map[string]*values),
	}
}

func (kvm kvmap) lookup(key []byte) *values {
	k := unsafeutil.BytesToString(key)
	if e, ok := kvm.kvs[k]; ok {
		return e
	}
	k = string(key)
	v := &values{
		k:  k,
		vs: make([]string, 0, kvm.mincap),
	}
	kvm.kvs[k] = v
	return v
}

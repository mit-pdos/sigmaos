package pathclnt

import (
	"fmt"
	"strings"
	"sync"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type RootMount struct {
	svcpn path.Path
	tree  path.Path
	uname string
}

type RootMountTable struct {
	sync.Mutex
	mounts map[string]*RootMount
}

func mkRootMountTable() *RootMountTable {
	mt := &RootMountTable{}
	mt.mounts = make(map[string]*RootMount)
	return mt
}

// XXX lookup should involve uname
func (rootmt *RootMountTable) lookup(name string) (*RootMount, *serr.Err) {
	rootmt.Lock()
	defer rootmt.Unlock()
	sm, ok := rootmt.mounts[name]
	if ok {
		return sm, nil
	}
	return nil, serr.MkErr(serr.TErrNotfound, fmt.Sprintf("%v (no root mount)", name))
}

func (rootmt *RootMountTable) add(uname string, svcpn, tree path.Path, mntname string) *serr.Err {
	rootmt.Lock()
	defer rootmt.Unlock()

	_, ok := rootmt.mounts[mntname]
	if ok {
		return serr.MkErr(serr.TErrExists, mntname)
	}
	rootmt.mounts[mntname] = &RootMount{svcpn: svcpn, tree: tree}
	return nil
}

func (rootmt *RootMountTable) isRootMount(mntname string) bool {
	rootmt.Lock()
	defer rootmt.Unlock()

	if mntname == sp.NAME {
		return true
	}
	_, ok := rootmt.mounts[mntname]
	return ok
}

func (pathc *PathClnt) resolveRoot(pn path.Path) *serr.Err {
	if len(pn) == 0 {
		return serr.MkErr(serr.TErrInval, fmt.Sprintf("empty path '%v' ", pn))
	}
	_, rest, err := pathc.mnt.resolve(pn, true)
	db.DPrintf(db.SVCMOUNT, "resolveRoot: %v r %v err %v\n", pn, rest, err)
	if err != nil && len(rest) >= 1 && pathc.rootmt.isRootMount(rest[0]) {
		if pn[0] == sp.NAME {
			return pathc.mountNamed(pn)
		} else {
			sm, err := pathc.rootmt.lookup(pn[0])
			if err != nil {
				return err
			}
			return pathc.mountRoot(sm.uname, sm.svcpn, sm.tree, pn[0])
		}
	}
	return nil
}

func (pathc *PathClnt) MakeRootMount(uname, pn, mntname string) error {
	if !strings.HasPrefix(pn, sp.NAME) {
		pn = sp.NAMED + pn
	}
	db.DPrintf(db.SVCMOUNT, "MakeRootMount: %v %v\n", pn, mntname)
	svc, rest, err := pathc.PathLastSymlink(pn)
	if err != nil {
		db.DPrintf(db.SVCMOUNT, "PathLastSymlink %v err %v\n", pn, err)
		return err
	}
	if err := pathc.mountRoot(uname, svc, rest, mntname); err != nil {
		return err
	}
	if err := pathc.rootmt.add(uname, svc, rest, mntname); err != nil {
		db.DPrintf(db.SVCMOUNT, "MakeRootMount: add %v err %v\n", svc, err)
		return err
	}
	return nil
}

func (pathc *PathClnt) mountRoot(uname string, svc, rest path.Path, mntname string) *serr.Err {
	db.DPrintf(db.SVCMOUNT, "mountRoot: %v %v %v %v\n", svc, rest, mntname)
	fid, _, err := pathc.mnt.resolve(svc, true)
	if err != nil {
		db.DPrintf(db.SVCMOUNT, "mountRoot: resolve %v err %v\n", svc, err)
		return err
	}
	ch := pathc.Lookup(fid)
	if ch == nil {
		db.DPrintf(db.SVCMOUNT, "mountRoot: lookup %v %v err nil\n", svc, fid)
	}
	addr := ch.Servers()
	if err := pathc.MountTree(uname, addr, rest.String(), mntname); err != nil {
		db.DPrintf(db.SVCMOUNT, "mountRoot: MountTree %v err %v\n", svc, err)
	}
	db.DPrintf(db.SVCMOUNT, "mountRoot: attached %v(%v):%v at %v\n", svc, addr, rest, mntname)
	return nil
}

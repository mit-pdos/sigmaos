package pathclnt

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type RootMount struct {
	svcpn     path.Path
	tree      path.Path
	principal *sp.Tprincipal
	closed    bool
}

type RootMountTable struct {
	sync.Mutex
	mounts map[string]*RootMount
}

func newRootMountTable() *RootMountTable {
	mt := &RootMountTable{}
	mt.mounts = make(map[string]*RootMount)
	mt.add(sp.NoPrincipal(), nil, nil, sp.NAME)
	return mt
}

// XXX lookup should involve principal
func (rootmt *RootMountTable) lookup(name string) (*RootMount, *serr.Err) {
	rootmt.Lock()
	defer rootmt.Unlock()
	sm, ok := rootmt.mounts[name]
	if ok {
		return sm, nil
	}
	return nil, serr.NewErr(serr.TErrNotfound, fmt.Sprintf("%v (no root mount)", name))
}

func (rootmt *RootMountTable) disconnect(name string) error {
	rootmt.Lock()
	defer rootmt.Unlock()
	sm, ok := rootmt.mounts[name]
	if ok {
		db.DPrintf(db.CRASH, "disconnect rootmnt %v", name)
		sm.closed = true
		return nil
	}
	return serr.NewErr(serr.TErrNotfound, fmt.Sprintf("%v (no root mount)", name))
}

func (rootmt *RootMountTable) add(principal *sp.Tprincipal, svcpn, tree path.Path, mntname string) *serr.Err {
	rootmt.Lock()
	defer rootmt.Unlock()

	_, ok := rootmt.mounts[mntname]
	if ok {
		return serr.NewErr(serr.TErrExists, mntname)
	}
	rootmt.mounts[mntname] = &RootMount{svcpn: svcpn, tree: tree}
	return nil
}

func (rootmt *RootMountTable) isRootMount(mntname string) bool {
	rootmt.Lock()
	defer rootmt.Unlock()

	_, ok := rootmt.mounts[mntname]
	return ok
}

// Resolve pn that names a server's root (e.g., name/ux, name)
func (pathc *PathClnt) resolveRoot(pn path.Path) (*serr.Err, bool) {
	db.DPrintf(db.PATHCLNT, "resolveRoot %v", pn)
	if len(pn) == 0 {
		return serr.NewErr(serr.TErrInval, fmt.Sprintf("empty path '%v' ", pn)), false
	}
	_, rest, err := pathc.mnt.resolve(pn, true)
	if err != nil && len(rest) >= 1 && pathc.rootmt.isRootMount(rest[0]) {
		sm, err := pathc.rootmt.lookup(pn[0])
		if err != nil {
			db.DPrintf(db.SVCMOUNT, "resolveRoot: lookup %v err %v\n", pn[0], err)
			return err, false
		}
		if sm.closed {
			db.DPrintf(db.CRASH, "resolveRoot: closed %v\n", pn[0])
			return serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("%v (closed root)", pn[0])), false
		}
		if pn[0] == sp.NAME {
			return pathc.mountNamed(pathc.pe.GetRealm(), sp.NAME), true
		} else {
			db.DPrintf(db.SVCMOUNT, "resolveRoot: remount %v at %v\n", sm, pn[0])
			// this may remount the service that this root is relying on
			// and repair this root mount
			if _, err := pathc.Stat(sm.svcpn.String()+"/", pathc.pe.GetPrincipal()); err != nil {
				var sr *serr.Err
				if errors.As(err, &sr) {
					return sr, false
				} else {
					return serr.NewErrError(err), false
				}
			}
			return pathc.mountRoot(sm.svcpn, sm.tree, pn[0]), true
		}
	}
	return nil, false
}

func (pathc *PathClnt) NewRootMount(principal *sp.Tprincipal, pn, mntname string) error {
	if !strings.HasPrefix(pn, sp.NAME) {
		pn = sp.NAMED + pn
	}
	db.DPrintf(db.SVCMOUNT, "NewRootMount: %v %v\n", pn, mntname)
	svc, rest, err := pathc.PathLastMount(pn, principal)
	if err != nil {
		db.DPrintf(db.SVCMOUNT, "NewRootMount %v err %v\n", pn, err)
		return err
	}
	if err := pathc.mountRoot(svc, rest, mntname); err != nil {
		return err
	}
	if err := pathc.rootmt.add(principal, svc, rest, mntname); err != nil {
		db.DPrintf(db.SVCMOUNT, "NewRootMount: add %v err %v\n", svc, err)
		return err
	}
	return nil
}

func (pathc *PathClnt) mountRoot(svc, rest path.Path, mntname string) *serr.Err {
	db.DPrintf(db.SVCMOUNT, "mountRoot: %v %v %v\n", svc, rest, mntname)
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
	if err := pathc.MountTree(pathc.pe.GetPrincipal(), addr, rest.String(), mntname); err != nil {
		db.DPrintf(db.SVCMOUNT, "mountRoot: MountTree %v err %v\n", svc, err)
	}
	db.DPrintf(db.SVCMOUNT, "mountRoot: attached %v(%v):%v at %v\n", svc, addr, rest, mntname)
	return nil
}

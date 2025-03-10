package mntclnt

import (
	"fmt"
	"strings"
	"sync"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type RootMount struct {
	svcpn     path.Tpathname
	tree      path.Tpathname
	principal *sp.Tprincipal
	closed    bool
}

func (rm *RootMount) String() string {
	return fmt.Sprintf("{pn %q tree %q p %v}", rm.svcpn, rm.tree, rm.principal)
}

type RootMountTable struct {
	sync.Mutex
	mounts map[sp.Tsigmapath]*RootMount
}

func newRootMountTable(pe *proc.ProcEnv) *RootMountTable {
	mt := &RootMountTable{}
	mt.mounts = make(map[sp.Tsigmapath]*RootMount)
	mt.add(sp.NoPrincipal(), nil, nil, sp.NAME)
	db.DPrintf(db.MOUNT, "Initial RootMntTable: %v", mt.mounts)
	return mt
}

func (rootmt *RootMountTable) String() string {
	return fmt.Sprintf("{mnts %v}", rootmt.mounts)
}

func (rootmt *RootMountTable) lookup(mntName sp.Tsigmapath) (*RootMount, *serr.Err) {
	rootmt.Lock()
	defer rootmt.Unlock()

	db.DPrintf(db.MOUNT, "RootMountTable.lookup mntName %v", mntName)
	if sm, ok := rootmt.mounts[mntName]; ok {
		db.DPrintf(db.MOUNT, "RootMountTable.lookup mntName %v found %v", mntName, sm)
		return sm, nil
	}
	db.DPrintf(db.MOUNT, "RootMountTable.lookup mntName %v no root mount", mntName)
	return nil, serr.NewErr(serr.TErrNotfound, fmt.Sprintf("%v (no root mount)", mntName))
}

func (rootmt *RootMountTable) disconnect(name sp.Tsigmapath) error {
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

func (rootmt *RootMountTable) add(principal *sp.Tprincipal, svcpn, tree path.Tpathname, mntname sp.Tsigmapath) *serr.Err {
	rootmt.Lock()
	defer rootmt.Unlock()

	_, ok := rootmt.mounts[mntname]
	if ok {
		return serr.NewErr(serr.TErrExists, mntname)
	}
	rootmt.mounts[mntname] = &RootMount{
		svcpn: svcpn,
		tree:  tree,
	}
	return nil
}

func (rootmt *RootMountTable) isRootMount(mntName sp.Tsigmapath) bool {
	rootmt.Lock()
	defer rootmt.Unlock()

	if _, ok := rootmt.mounts[mntName]; ok {
		db.DPrintf(db.MOUNT, "isRootMount mntName %v true", mntName)
		return true
	}
	db.DPrintf(db.MOUNT, "isRootMount mntName %v false", mntName)
	return false
}

// Resolve pn that names a server's root (e.g., name/ux, name)
func (mc *MntClnt) resolveRoot(pn path.Tpathname) (*serr.Err, bool) {
	db.DPrintf(db.MOUNT, "resolveRoot %v", pn)
	if len(pn) == 0 {
		return serr.NewErr(serr.TErrInval, fmt.Sprintf("empty path '%v' ", pn)), false
	}
	_, rest, err := mc.ResolveMnt(pn, true)
	if err != nil && len(rest) >= 1 && mc.rootmt.isRootMount(rest[0]) {
		sm, err := mc.rootmt.lookup(pn[0])
		if err != nil {
			db.DPrintf(db.MOUNT, "resolveRoot: lookup %v err %v\n", pn[0], err)
			return err, false
		}
		if sm.closed {
			db.DPrintf(db.CRASH, "resolveRoot: closed %v\n", pn[0])
			return serr.NewErr(serr.TErrUnreachable, fmt.Sprintf("%v (closed root)", pn[0])), false
		}
		if pn[0] == sp.NAME {
			return mc.mountNamed(mc.pe.GetRealm(), sp.NAME, ""), true
		} else {
			db.DPrintf(db.MOUNT, "resolveRoot: remount %v at %v\n", sm, pn[0])
			// this may remount the service that this root is relying on
			// and repair this root mount
			if _, err := mc.pathc.Stat(sm.svcpn.String()+"/", mc.pe.GetPrincipal()); err != nil {
				if sr, ok := serr.IsErr(err); ok {
					return sr, false
				} else {
					return serr.NewErrError(err), false
				}
			}
			return mc.mountRoot(sm.svcpn, sm.tree, pn[0]), true
		}
	}
	return nil, false
}

func (mc *MntClnt) NewRootMount(principal *sp.Tprincipal, pn, mntname sp.Tsigmapath) error {
	if !strings.HasPrefix(pn, sp.NAME) {
		pn = sp.NAMED + pn
	}
	db.DPrintf(db.MOUNT, "NewRootMount: %v %v\n", pn, mntname)
	svc, rest, err := mc.PathLastMount(pn, principal)
	if err != nil {
		db.DPrintf(db.MOUNT, "NewRootMount %v err %v\n", pn, err)
		return err
	}
	if err := mc.mountRoot(svc, rest, mntname); err != nil {
		return err
	}
	if err := mc.rootmt.add(principal, svc, rest, mntname); err != nil {
		db.DPrintf(db.MOUNT, "NewRootMount: add %v err %v\n", svc, err)
		return err
	}
	return nil
}

func (mc *MntClnt) mountRoot(svc, rest path.Tpathname, mntname sp.Tsigmapath) *serr.Err {
	db.DPrintf(db.MOUNT, "mountRoot: %v %v %v\n", svc, rest, mntname)
	fid, _, err := mc.ResolveMnt(svc, true)
	if err != nil {
		db.DPrintf(db.MOUNT, "mountRoot: resolve %v err %v\n", svc, err)
		return err
	}
	ch := mc.fidc.Lookup(fid)
	if ch == nil {
		db.DPrintf(db.MOUNT, "mountRoot: lookup %v %v err nil\n", svc, fid)
	}
	ep := ch.Endpoint()
	if err := mc.MountTree(mc.pe.GetSecrets(), ep, rest.String(), mntname); err != nil {
		db.DPrintf(db.MOUNT, "mountRoot: MountTree %v err %v\n", svc, err)
	}
	db.DPrintf(db.MOUNT, "mountRoot: attached %v(%v):%v at %v\n", svc, ep, rest, mntname)
	return nil
}

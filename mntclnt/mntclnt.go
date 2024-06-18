package mntclnt

import (
	"fmt"
	"time"

	db "sigmaos/debug"
	"sigmaos/fidclnt"
	"sigmaos/netproxyclnt"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sigmaos"
	sp "sigmaos/sigmap"
)

type MntClnt struct {
	ndMntCache *NamedEndpointCache
	mnt        *MntTable
	rootmt     *RootMountTable
	pe         *proc.ProcEnv
	npc        *netproxyclnt.NetProxyClnt
	cid        sp.TclntId
	fidc       *fidclnt.FidClnt
	pathc      sigmaos.PathClntAPI
}

func NewMntClnt(pathc sigmaos.PathClntAPI, fidc *fidclnt.FidClnt, cid sp.TclntId, pe *proc.ProcEnv, npc *netproxyclnt.NetProxyClnt) *MntClnt {
	mc := &MntClnt{
		cid:        cid,
		mnt:        newMntTable(),
		ndMntCache: newNamedEndpointCache(pe),
		rootmt:     newRootMountTable(pe),
		pe:         pe,
		npc:        npc,
		fidc:       fidc,
		pathc:      pathc,
	}
	return mc
}

func (mc *MntClnt) String() string {
	return fmt.Sprintf("{mnt %v ndMntCache %v rootmt %v}", mc.mnt, mc.ndMntCache, mc.rootmt)
}

func (mc *MntClnt) Mounts() []string {
	return mc.mnt.mountedPaths()
}

func (mc *MntClnt) Resolve(p path.Tpathname, principal *sp.Tprincipal, resolve bool) (sp.Tfid, path.Tpathname, *serr.Err) {
	if err, b := mc.resolveRoot(p); err != nil {
		db.DPrintf(db.ALWAYS, "%v: resolveRoot %v err %v b %v\n", mc.cid, p, err, b)
	}
	return mc.mnt.resolveMnt(p, resolve)
}

func (mc *MntClnt) ResolveMnt(p path.Tpathname, resolve bool) (sp.Tfid, path.Tpathname, *serr.Err) {
	return mc.mnt.resolveMnt(p, resolve)
}

// XXX use MountedAt
func (mc *MntClnt) LastMount(pn string, principal *sp.Tprincipal) (path.Tpathname, path.Tpathname, error) {
	p, err := serr.PathSplitErr(pn)
	if err != nil {
		return nil, nil, err
	}
	_, left, err := mc.Resolve(p, principal, path.EndSlash(pn))
	if err != nil {
		db.DPrintf(db.MOUNT_ERR, "%v: resolve  %v err %v\n", mc.cid, pn, err)
		return nil, nil, err
	}
	p = p[0 : len(p)-len(left)]
	return p, left, nil
}

func (mc *MntClnt) ResolveRoot(pn path.Tpathname) (*serr.Err, bool) {
	s := time.Now()
	err, ok := mc.resolveRoot(pn)
	db.DPrintf(db.WALK_LAT, "ResolveRoot %v %v lat %v\n", mc.cid, pn, time.Since(s))
	if err != nil {
		db.DPrintf(db.MOUNT_ERR, "ResolveRoot unreachable %v err %v\n", pn, err)
	}
	return err, ok
}

// Return path through the last mounted server and the rest of the
// path on the server.
func (mc *MntClnt) PathLastMount(pn string, principal *sp.Tprincipal) (path.Tpathname, path.Tpathname, error) {
	// Automount the longest prefix of pn; if pn exist, then the
	// server holding the directory/file correspending to pn.
	if _, err := mc.pathc.Stat(pn+"/", principal); err != nil {
		db.DPrintf(db.MOUNT_ERR, "%v: Stat %v err %v\n", mc.cid, pn, err)
	}
	return mc.LastMount(pn, principal)
}

func (mc *MntClnt) AutoMount(secrets map[string]*sp.SecretProto, ep *sp.Tendpoint, path path.Tpathname) *serr.Err {
	db.DPrintf(db.MOUNT, "%v: automount %v to %v\n", mc.cid, ep, path)
	var fid sp.Tfid
	var err *serr.Err
	s := time.Now()
	fid, err = mc.fidc.Attach(secrets, mc.cid, ep, path.String(), ep.Root)
	if err != nil {
		db.DPrintf(db.MOUNT_ERR, "Attach error: %v", err)
		return err
	}
	db.DPrintf(db.WALK_LAT, "%v: automount pn '%v' Attach lat %v\n", mc.cid, path, time.Since(s))
	db.DPrintf(db.SPAWN_LAT, "%v: automount pn '%v' Attach lat %v\n", mc.cid, path, time.Since(s))
	err = mc.mount(fid, path.String())
	if err != nil {
		return err
	}
	return nil
}

func (mc *MntClnt) MountTree(secrets map[string]*sp.SecretProto, ep *sp.Tendpoint, tree, mntname string) error {
	db.DPrintf(db.MOUNT, "MountTree [%v]:%q mnt %v", ep, tree, mntname)
	ok, err := mc.isMountedAt(mntname)
	db.DPrintf(db.MOUNT, "isMounted [%v] %t %v", mntname, ok, err)
	if err != nil {
		return nil
	}
	if ok {
		return nil
	}
	s := time.Now()
	if fid, err := mc.fidc.Attach(secrets, mc.cid, ep, "", tree); err == nil {
		db.DPrintf(db.SPAWN_LAT, "Attach %v lat %v", mntname, time.Since(s))
		return mc.Mount(fid, mntname)
	} else {
		db.DPrintf(db.MOUNT_ERR, "%v: MountTree Attach [%v]/%v err %v", mc.cid, ep, tree, err)
		return err
	}
}

func (mc *MntClnt) Mount(fid sp.Tfid, path string) error {
	if err := mc.mount(fid, path); err != nil {
		return err
	}
	return nil
}

// Detach from server
func (mc *MntClnt) Detach(pn string) error {
	p, err := serr.PathSplitErr(pn)
	if err != nil {
		return err
	}
	fid, _, err := mc.mnt.umount(p, true)
	if err != nil {
		db.DPrintf(db.TEST, "%v: Detach %q err %v\n", mc.cid, pn, err)
		return err
	}
	defer mc.fidc.Free(fid)
	if err := mc.fidc.Detach(fid, mc.cid); err != nil {
		return err
	}
	return nil
}

func (mc *MntClnt) UmountPrefix(path []string) *serr.Err {
	if fid, _, err := mc.mnt.umount(path, false); err != nil {
		return err
	} else {
		mc.fidc.Free(fid)
		return nil
	}
}

func (mc *MntClnt) MountedPaths() []string {
	return mc.mnt.mountedPaths()
}

// Disconnect client from server permanently to simulate network
// partition to server that exports pn
func (mc *MntClnt) Disconnect(pn string, fids []sp.Tfid) error {
	db.DPrintf(db.CRASH, "Disconnect %v mnts %v\n", pn, mc.MountedPaths())
	p, err := serr.PathSplitErr(pn)
	if err != nil {
		return err
	}
	_, mntp := mc.mnt.isMountedAt(p)
	for _, fid := range fids {
		ch := mc.fidc.Lookup(fid)
		if ch != nil {
			if p.IsParent(ch.Path()) {
				db.DPrintf(db.CRASH, "fid disconnect fid %v %v %v\n", fid, ch, mntp)
				mc.fidc.Disconnect(fid)
			}
		}
	}
	mc.rootmt.disconnect(mntp.String())
	fid, err := mc.mnt.disconnect(mntp)
	if err != nil {
		return err
	}
	mc.fidc.Disconnect(fid)
	return nil
}

func (mc *MntClnt) isMountedAt(pn string) (bool, error) {
	p, err := serr.PathSplitErr(pn)
	if err != nil {
		return false, err
	}
	ok, pn0 := mc.mnt.isMountedAt(p)
	if ok && pn0 == nil {
		return true, nil
	}
	return false, nil
}

func (mc *MntClnt) mount(fid sp.Tfid, pn string) *serr.Err {
	p, err := serr.PathSplitErr(pn)
	if err != nil {
		return err
	}
	if err := mc.mnt.add(p, fid); err != nil {
		if err.Code() == serr.TErrExists {
			// Another thread may already have mounted
			// path; clunk the fid and don't return an
			// error.
			mc.fidc.Clunk(fid)
			return nil
		} else {
			return err
		}
	}
	return nil
}

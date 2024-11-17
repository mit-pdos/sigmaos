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
	sp "sigmaos/sigmap"
)

type MntClntAPI interface {
	GetFile(pn string, principal *sp.Tprincipal, mode sp.Tmode, off sp.Toffset, cnt sp.Tsize, f *sp.Tfence) ([]byte, error)
	Stat(pn string, principal *sp.Tprincipal) (*sp.Stat, error)
}

type MntClnt struct {
	ndMntCache *NamedEndpointCache
	mnt        *MntTable
	rootmt     *RootMountTable
	pe         *proc.ProcEnv
	npc        *netproxyclnt.NetProxyClnt
	cid        sp.TclntId
	fidc       *fidclnt.FidClnt
	pathc      MntClntAPI
}

func NewMntClnt(pathc MntClntAPI, fidc *fidclnt.FidClnt, cid sp.TclntId, pe *proc.ProcEnv, npc *netproxyclnt.NetProxyClnt) *MntClnt {
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

func (mc *MntClnt) Resolve(p path.Tpathname, principal *sp.Tprincipal, resolve bool) (sp.Tfid, path.Tpathname, *serr.Err) {
	if err, ok := mc.resolveRoot(p); err != nil {
		db.DPrintf(db.ALWAYS, "%v: resolveRoot %v err %v b %v\n", mc.cid, p, err, ok)
	}
	return mc.ResolveMnt(p, resolve)
}

func (mc *MntClnt) ResolveMnt(p path.Tpathname, resolve bool) (sp.Tfid, path.Tpathname, *serr.Err) {
	for {
		pnt, path, err := mc.mnt.resolveMnt(p, resolve)
		db.DPrintf(db.MOUNT, "%v: resolveMnt path %v resolve %v mnt %v", mc.cid, path, resolve, pnt)
		if err != nil {
			return sp.NoFid, path, err
		}
		fid, ok := pnt.getFid()
		if ok {
			return fid, path, nil
		}
		// MountTree() is trying (or will try) to mount pnt and may
		// succeed or fail; retry resolveMnt.  The retry won't happen
		// often since MountTree() will lock pnt while mounting.
		db.DPrintf(db.TEST, "%v: resolveMnt try again path %v resolve %t", mc.cid, path, resolve)
	}
}

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

func (mc *MntClnt) MountTree(secrets map[string]*sp.SecretProto, ep *sp.Tendpoint, tree, mntname string) error {
	pn, err := serr.PathSplitErr(mntname)
	if err != nil {
		return err
	}
	db.DPrintf(db.MOUNT, "%v: MountTree [%v]:%q mnt %v", mc.cid, ep, tree, mntname)
	pnt, err := mc.mnt.lookupAlloc(pn, sp.NoFid)
	if err != nil {
		return err
	}

	pnt.Lock()
	defer pnt.Unlock()

	db.DPrintf(db.MOUNT, "%v: isAttached? [%v] %t err %v", mc.cid, ep, pnt.isAttached(), err)

	if pnt.isAttached() {
		return nil
	}

	db.DPrintf(db.MOUNT, "MountTree [%v]:%q attach %v", ep, tree, mntname)

	s := time.Now()
	fid, err := mc.fidc.Attach(secrets, mc.cid, ep, pn, tree)
	if err != nil {
		db.DPrintf(db.MOUNT_ERR, "%v: MountTree Attach [%v]/%v err %v", mc.cid, ep, tree, err)
		mc.mnt.umount(pn, true)
	}
	db.DPrintf(db.MOUNT, "%v: MountTree pn %q err %v Attach lat %v\n", mc.cid, mntname, err, time.Since(s))

	if err != nil {
		return err
	}
	pnt.fid = fid
	return nil
}

// Detach from server
func (mc *MntClnt) Detach(pn string) error {
	p, err := serr.PathSplitErr(pn)
	if err != nil {
		return err
	}
	pnt, err := mc.mnt.umount(p, true)
	if err != nil {
		db.DPrintf(db.TEST, "%v: Detach %q err %v\n", mc.cid, pn, err)
		return err
	}
	fid, ok := pnt.getFid()
	if ok {
		defer mc.fidc.Free(fid)
		if err := mc.fidc.Detach(fid, mc.cid); err != nil {
			return err
		}
	}
	return nil
}

func (mc *MntClnt) UmountPrefix(path []string) *serr.Err {
	if pnt, err := mc.mnt.umount(path, false); err != nil {
		return err
	} else {
		fid, ok := pnt.getFid()
		if ok {
			mc.fidc.Free(fid)
		}
		return nil
	}
}

func (mc *MntClnt) MountedPaths() []string {
	return mc.mnt.mountedPaths()
}

// Disconnect client from server permanently to simulate network
// partition to server that exports pn
func (mc *MntClnt) Disconnect(pn string) error {
	p, err := serr.PathSplitErr(pn)
	if err != nil {
		return err
	}
	pnt, ok := mc.mnt.isMountedAt(p)
	if ok {
		db.DPrintf(db.CRASH, "Disconnect %v pnt %v\n", pn, pnt)
		mc.fidc.DisconnectAll(pnt.fid)
		pnt.disconnect()
	} else {
		return serr.NewErr(serr.TErrUnreachable, pnt.path)
	}
	mc.rootmt.disconnect(pnt.path.String())
	return nil
}

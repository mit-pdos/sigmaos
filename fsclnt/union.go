package fsclnt

import (
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/protclnt"
)

func (fsc *FidClient) walkUnion(fid np.Tfid, path []string, todo int) (np.Tfid, int, *np.Err) {
	fid2 := fsc.fids.allocFid()
	i := len(path) - todo
	err := fsc.unionLookup(fid, fid2, path[i])
	if err != nil {
		return np.NoFid, 0, err
	}
	return fid2, todo - 1, nil
}

func (fsc *FidClient) unionMatch(q, name string) bool {
	switch q {
	case "~any":
		return true
	case "~ip":
		ip, err := LocalIP()
		if err != nil {
			return false
		}
		// XXX need to match on ip, but for now at least
		// check that name is a remote target
		if IsRemoteTarget(name) && ip == ip {
			return true
		}
		return false
	default:
		return true
	}
	return true
}

func (fsc *FidClient) unionScan(pc *protclnt.ProtClnt, fid, fid2 np.Tfid, dirents []*np.Stat, q string) (bool, *np.Err) {
	for _, de := range dirents {
		// Read the link
		fid3 := fsc.fids.allocFid()
		_, err := pc.Walk(fid, fid3, []string{de.Name})
		if err != nil {
			return false, err
		}
		// XXX defer clunk fid3
		target, err := fsc.readlink(pc, fid3)
		if err != nil {
			return false, err
		}
		if fsc.unionMatch(q, target) {
			reply, err := pc.Walk(fid, fid2, []string{de.Name})
			if err != nil {
				return false, err
			}
			fsc.fids.addFid(fid2, fsc.fids.path(fid).copyPath())
			fsc.fids.path(fid2).addn(reply.Qids, []string{de.Name})
			return true, nil
		}
	}
	return false, nil
}

func (fsc *FidClient) unionLookup(fid, fid2 np.Tfid, q string) *np.Err {
	pc := fsc.fids.clnt(fid)
	_, err := pc.Open(fid, np.OREAD)
	if err != nil {
		return err
	}
	off := np.Toffset(0)
	for {
		reply, err := pc.Read(fid, off, 1024)
		if err != nil {
			return err
		}
		if len(reply.Data) == 0 {
			return np.MkErr(np.TErrNotfound, q)
		}
		dirents, err := npcodec.Byte2Dir(reply.Data)
		if err != nil {
			return err
		}
		ok, err := fsc.unionScan(pc, fid, fid2, dirents, q)
		if err != nil {
			return err
		}
		if ok {
			break
		}
		off += 1024
	}
	return nil
}

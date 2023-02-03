package pathclnt

import (
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"strings"
)

func (pathc *PathClnt) walkSymlink1(fid sp.Tfid, resolved, left path.Path) (path.Path, *serr.Err) {
	// XXX change how we readlink; getfile?
	target, err := pathc.readlink(fid)
	if err != nil {
		db.DPrintf(db.WALK, "walksymlink1 %v err %v\n", fid, err)
		return left, err
	}
	var p path.Path
	mnt, error := sp.MkMount(target)
	if error == nil {
		db.DPrintf(db.WALK, "walksymlink1 %v mnt %v err %v\n", fid, mnt, err)
		err := pathc.autoMount(pathc.FidClnt.Lookup(fid).Uname(), mnt, resolved)
		if err != nil {
			db.DPrintf(db.WALK, "automount %v %v err %v\n", resolved, mnt, err)
			return left, err
		}
		p = append(resolved, left...)
	} else {
		db.DPrintf(db.WALK, "walksymlink1 %v MkMount err %v\n", fid, err)
		p = append(path.Split(string(target)), left...)
	}
	return p, nil
}

func (pathc *PathClnt) autoMount(uname string, mnt sp.Tmount, path path.Path) *serr.Err {
	var fid sp.Tfid
	var err *serr.Err
	addr := mnt.Addr[0]
	if strings.HasPrefix(addr, "10.0.") {
		addr = "127.0.0.1:1112"
	}
	db.DPrintf(db.PATHCLNT, "automount %v (%s) to %v\n", mnt, addr, path)
	fid, err = pathc.Attach(uname, sp.Taddrs{addr}, path.String(), mnt.Root)
	if err != nil {
		db.DPrintf(db.PATHCLNT, "Attach error: %v", err)
		return err
	}
	err = pathc.mount(fid, path.String())
	if err != nil {
		return err
	}
	return nil
}

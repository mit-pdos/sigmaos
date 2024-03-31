package pathclnt

import (
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func (pathc *PathClnt) walkSymlink1(fid sp.Tfid, resolved, left path.Path) (path.Path, *serr.Err) {
	// XXX change how we readlink; getfile?
	target, err := pathc.readlink(fid)
	if err != nil {
		db.DPrintf(db.WALK, "walksymlink1 %v err %v\n", fid, err)
		return left, err
	}
	var p path.Path
	mnt, error := sp.NewMount(target)
	if error == nil {
		db.DPrintf(db.WALK, "walksymlink1 %v mnt %v err %v\n", fid, mnt, err)
		err := pathc.autoMount(pathc.FidClnt.Lookup(fid).Principal(), mnt, resolved)
		if err != nil {
			db.DPrintf(db.WALK, "automount %v %v err %v\n", resolved, mnt, err)
			return left, err
		}
		p = append(resolved, left...)
	} else {
		db.DPrintf(db.WALK, "walksymlink1 %v NewMount err %v\n", fid, err)
		p = append(path.Split(string(target)), left...)
	}
	return p, nil
}

func (pathc *PathClnt) autoMount(principal *sp.Tprincipal, mnt *sp.Tmount, path path.Path) *serr.Err {
	var fid sp.Tfid
	var err *serr.Err

	db.DPrintf(db.PATHCLNT, "automount %v to %v\n", mnt, path)
	fid, err = pathc.Attach(principal, pathc.cid, mnt, path.String(), mnt.Root)
	if err != nil {
		db.DPrintf(db.PATHCLNT_ERR, "Attach error: %v", err)
		return err
	}
	err = pathc.mount(fid, path.String())
	if err != nil {
		return err
	}
	return nil
}

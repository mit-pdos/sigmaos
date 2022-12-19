package pathclnt

import (
	db "sigmaos/debug"
	"sigmaos/fcall"
	"sigmaos/path"
	sp "sigmaos/sigmap"
)

func (pathc *PathClnt) walkSymlink1(fid sp.Tfid, resolved, left path.Path) (path.Path, *fcall.Err) {
	// XXX change how we readlink; getfile?
	target, err := pathc.readlink(fid)
	if err != nil {
		db.DPrintf("WALK", "walksymlink1 %v err %v\n", fid, err)
		return left, err
	}
	var p path.Path
	mnt, error := sp.MkMount(target)
	if error == nil {
		db.DPrintf("WALK", "walksymlink1 %v mnt %v err %v\n", fid, mnt, err)
		err := pathc.autoMount(pathc.FidClnt.Lookup(fid).Uname(), mnt, resolved)
		if err != nil {
			db.DPrintf("WALK", "automount %v %v err %v\n", resolved, mnt, err)
			return left, err
		}
		p = append(resolved, left...)
	} else {
		db.DPrintf("WALK", "walksymlink1 %v MkMount err %v\n", fid, err)
		p = append(path.Split(string(target)), left...)
	}
	return p, nil
}

func (pathc *PathClnt) autoMount(uname string, mnt sp.Tmount, path path.Path) *fcall.Err {
	db.DPrintf("PATHCLNT0", "automount %v to %v\n", mnt, path)
	var fid sp.Tfid
	var err *fcall.Err
	fid, err = pathc.Attach(uname, mnt.AddrIP4, path.String(), mnt.Root)
	if err != nil {
		db.DPrintf("PATHCLNT", "Attach error: %v", err)
		return err
	}
	err = pathc.mount(fid, path.String())
	if err != nil {
		return err
	}
	return nil
}

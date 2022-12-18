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
	db.DPrintf("WALK", "walksymlink1 %v target %v err %v\n", fid, target, err)
	if err != nil {
		return left, err
	}
	var p path.Path
	if sp.IsRemoteTarget(target) {
		err := pathc.autoMount(pathc.FidClnt.Lookup(fid).Uname(), target, resolved)
		if err != nil {
			db.DPrintf("WALK", "automount %v %v err %v\n", resolved, target, err)
			return left, err
		}
		p = append(resolved, left...)
	} else {
		p = append(path.Split(target), left...)
	}
	return p, nil
}

func (pathc *PathClnt) autoMount(uname string, target string, path path.Path) *fcall.Err {
	db.DPrintf("PATHCLNT0", "automount %v to %v\n", target, path)
	var fid sp.Tfid
	var err *fcall.Err
	if sp.IsReplicated(target) {
		addrs, r := sp.SplitTargetReplicated(target)
		fid, err = pathc.Attach(uname, addrs, path.String(), r.String())
	} else {
		addr, r := sp.SplitTarget(target)
		db.DPrintf("PATHCLNT0", "Split target: %v", r)
		fid, err = pathc.Attach(uname, []string{addr}, path.String(), r.String())
	}
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

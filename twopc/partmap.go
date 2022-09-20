package twopc

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/pathclnt"
)

type FlwsMap struct {
	*fslib.FsLib
	flws map[string]bool
}

func mkFlwsMap(fsl *fslib.FsLib, f []string) *FlwsMap {
	flws := &FlwsMap{}
	flws.FsLib = fsl
	flws.flws = make(map[string]bool)
	for _, fw := range f {
		if _, ok := flws.flws[fw]; !ok {
			flws.flws[fw] = true
		}
	}
	return flws
}

func mkFlwsMapStatus(fsl *fslib.FsLib, dir string) *FlwsMap {
	fw := mkFlwsMap(fsl, nil)
	sts, err := fw.GetDir(dir)
	if err != nil {
		return nil
	}
	for _, st := range sts {
		fw.flws[st.Name] = true
	}
	return fw
}

func (fw *FlwsMap) String() string {
	return fmt.Sprintf("%v", fw.flws)
}

func (fw *FlwsMap) len() int {
	return len(fw.flws)
}

// XXX maybe should be OpenWatch(fn, f)?
func (fw *FlwsMap) setStatusWatches(dir string, f pathclnt.Watch) {
	for flw, _ := range fw.flws {
		fn := dir + flw
		// set watch for existence of fn, which indicates fn
		// has prepared/committed
		_, err := fw.GetFileWatch(fn)
		if err == nil {
			db.DFatalf("COORD: set status watch failed %v", err)
		}
		// XXX error could be something else than "file not found",
		// in which case we want to fail?
	}
}

func (fw *FlwsMap) setFlwsWatches(f pathclnt.Watch) {
	for flw, _ := range fw.flws {
		// set watch for KV, in case it crashes during 2PC
		err := fw.SetRemoveWatch(DIR2PC+"/"+flw, f)
		if err != nil {
			db.DFatalf("COORD: set follower watch failed %v", err)
		}
	}
}

func (fw *FlwsMap) doCommit(prepared *FlwsMap) bool {
	if prepared == nil || prepared.len() != fw.len() {
		return false
	}
	for fw, _ := range fw.flws {
		if _, ok := prepared.flws[fw]; !ok {
			return false
		}
	}
	return true
}

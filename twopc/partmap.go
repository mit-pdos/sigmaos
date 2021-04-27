package twopc

import (
	"fmt"
	"log"

	"ulambda/fsclnt"
	"ulambda/fslib"
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
	sts, err := fw.ReadDir(dir)
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

func (fw *FlwsMap) setStatusWatches(dir string, f fsclnt.Watch) {
	for flw, _ := range fw.flws {
		fn := dir + flw
		// set watch for existence of fn, which indicates fn
		// has prepared/committed
		_, err := fw.ReadFileWatch(fn, f)
		if err == nil {
			log.Fatalf("COORD: set status watch failed %v", err)
		}
	}
}

func (fw *FlwsMap) setFlwsWatches(f fsclnt.Watch) {
	for flw, _ := range fw.flws {
		// set watch for KV, in case it crashes during 2PC
		err := fw.SetRemoveWatch(DIR2PC+"/"+flw, f)
		if err != nil {
			log.Fatalf("COORD: set follower watch failed %v", err)
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

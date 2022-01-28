package fenceclnt

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	db "ulambda/debug"
	"ulambda/fence"
	"ulambda/fslib"
	np "ulambda/ninep"
)

// for test
const (
	FENCE_DIR = "name/fence"
)

//
// XXX FIX ME
//
// Support for fences, which consists of pathname and a Qid of that
// pathname. A proc can fence all servers it has an open session with.
// When receiving an operation, the servers check if the fence is
// still valid (by checking the Qid of the file in the fence).  If the
// Qid is unchanged from the registered fence, they allow the
// operation; otherwise, they reject the operation.
//
// Procs uses FenceClnt to interact with fences, which they can use in
// two in two ways: write fences and read fences.  Write fences are
// for, for example, coordinators to obtain an exclusive FenceClnt so
// that only one coorditor is active.  The write fence maybe
// invalidated anytime, for example, by a network partition, which
// allows another a new coordinator to get the FenceClnt.  The old
// coordinator won't be able to perform operations at any server,
// because its fence will invalid as soon as the new coordinator
// obtains the write fence.
//
// Multiple procs may have a read fence on, for example, a FenceClnt
// that represents a configuration file.  A read fence maybe
// invalidated by a proc that modifies the configuration file,
// signaling to the reader they should reread the configuration
// file. Operations in flight to any server will be rejected by those
// servers because the read fence is invalid.
//

type FenceClnt struct {
	fenceName string // pathname for the fence file
	*fslib.FsLib
	perm np.Tperm
	mode np.Tmode
}

func MakeFenceClnt(fsl *fslib.FsLib, name string, perm np.Tperm) *FenceClnt {
	f := &FenceClnt{}
	f.fenceName = name
	f.FsLib = fsl
	f.perm = perm
	return f
}

func (f *FenceClnt) registerFence(mode np.Tmode) error {
	f.mode = mode
	st, err := f.Stat(f.fenceName)
	if err != nil {
		log.Printf("%v: Stat %v err %v", db.GetName(), st, err)
		return err
	}
	err = f.RegisterFence(fence.MakeFence(f.fenceName, st.Qid))
	if err != nil {
		log.Printf("%v: registerRFence %v err %v", db.GetName(), f.fenceName, err)
		return err
	}
	return nil
}

func (f *FenceClnt) updateFence() error {
	st, err := f.Stat(f.fenceName)
	if err != nil {
		log.Printf("%v: Stat %v err %v", db.GetName(), st, err)
		return err
	}
	err = f.UpdateFence(fence.MakeFence(f.fenceName, st.Qid))
	if err != nil {
		log.Printf("%v: registerRFence %v err %v", db.GetName(), f.fenceName, err)
		return err
	}
	return nil
}

//
// Acquire fences for writing
//

// Wait to obtain a write fence and initialize the file with b
// XXX cleanup on failure
// XXX create and write atomic
func (f *FenceClnt) AcquireFenceW(b []byte) error {
	fd, err := f.Create(f.fenceName, f.perm|np.DMTMP, np.OWRITE|np.OWATCH)
	if err != nil {
		log.Printf("%v: Makefile %v err %v", db.GetName(), f.fenceName, err)
		return err
	}
	_, err = f.Write(fd, b)
	if err != nil {
		log.Printf("%v: write %v err %v", db.GetName(), f.fenceName, err)
		return err
	}
	f.Close(fd)
	return f.registerFence(np.OWRITE)
}

func (f *FenceClnt) ReleaseFence() error {
	if f.mode == np.OWRITE {
		err := f.Remove(f.fenceName)
		if err != nil {
			log.Printf("%v: Remove %v err %v", db.GetName(), f.fenceName, err)
			return err
		}
	}
	return f.DeregisterFence(f.fenceName)
}

// Make the fence file
func (f *FenceClnt) MakeFenceFile(b []byte) error {
	err := f.MakeFile(f.fenceName, 0777|np.DMTMP, np.OWRITE, b)
	if err != nil {
		log.Printf("%v: MakeFenceFile %v err %v", db.GetName(), f.fenceName, err)
		return err
	}
	// XXX notify fence has changed
	return nil
}

// Update the fence file
func (f *FenceClnt) SetFenceFile(b []byte) error {
	_, err := f.SetFile(f.fenceName, b)
	if err != nil {
		log.Printf("%v: SetFenceFile %v err %v", db.GetName(), f.fenceName, err)
		return err
	}
	return f.updateFence()
}

func (f *FenceClnt) MakeFenceFileJson(i interface{}) error {
	b, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("Marshal error %v", err)
	}
	return f.MakeFenceFile(b)
}

func (f *FenceClnt) MakeFenceFileFrom(from string) error {
	err := f.Rename(from, f.fenceName)
	if err != nil {
		log.Printf("%v: Rename %v to %v err %v", db.GetName(), from, f.fenceName, err)
		return err
	}
	return f.updateFence()
}

//
// Acquire fence in "read" mode
//

func (f *FenceClnt) AcquireFenceR() ([]byte, error) {
	ch := make(chan bool)
	for {
		// log.Printf("%v: file watch %v\n", db.GetName(), f.fenceName)
		b, err := f.ReadFileWatch(f.fenceName, func(string, error) {
			ch <- true
		})
		if err != nil && strings.HasPrefix(err.Error(), "file not found") {
			// log.Printf("%v: file watch wait %v\n", db.GetName(), f.fenceName)
			<-ch
		} else if err != nil {
			return nil, err
		} else {
			// log.Printf("%v: file watch return %v\n", db.GetName(), f.fenceName)
			return b, f.registerFence(np.OREAD)
		}
	}
}

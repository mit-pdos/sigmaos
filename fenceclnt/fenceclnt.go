package fenceclnt

import (
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
	f    *fence.Fence
}

func MakeFenceClnt(fsl *fslib.FsLib, name string, perm np.Tperm) *FenceClnt {
	fc := &FenceClnt{}
	fc.fenceName = name
	fc.FsLib = fsl
	fc.perm = perm
	return fc
}

func (fc *FenceClnt) Fence() *fence.Fence {
	return fc.f
}

func (fc *FenceClnt) Name() string {
	return fc.fenceName
}

func (fc *FenceClnt) registerFence(mode np.Tmode) error {
	fence, err := fc.MakeFence(fc.fenceName, mode)
	if err != nil {
		log.Printf("%v: MakeFence %v err %v", db.GetName(), fc.fenceName, err)
		return err
	}
	log.Printf("%v: MakeFence %v fence %v", db.GetName(), fc.fenceName, fence)
	if fc.f == nil {
		fc.mode = mode
		err = fc.RegisterFence(fence, fence.Qid)
	} else {
		err = fc.UpdateFence(fence, fc.f.Qid)
	}
	if err != nil {
		log.Printf("%v: registerFence %v err %v", db.GetName(), fc.fenceName, err)
		return err
	}
	fc.f = fence
	return nil
}

// Wait to obtain a write fence and initialize the file with b
// XXX cleanup on failure
// XXX create and write atomic
func (fc *FenceClnt) AcquireFenceW(b []byte) error {
	fd, err := fc.Create(fc.fenceName, fc.perm|np.DMTMP, np.OWRITE|np.OWATCH)
	if err != nil {
		log.Printf("%v: Create %v err %v", db.GetName(), fc.fenceName, err)
		return err
	}

	_, err = fc.Write(fd, b)
	if err != nil {
		log.Printf("%v: Write %v err %v", db.GetName(), fc.fenceName, err)
		return err
	}
	fc.Close(fd)
	return fc.registerFence(np.OWRITE)
}

func (fc *FenceClnt) ReleaseFence() error {
	// First deregister fence
	if fc.f == nil {
		log.Fatalf("%v: FATAL ReleaseFence %v\n", db.GetName(), fc.fenceName)
	}
	err := fc.DeregisterFence(fc.f.Fence)
	if err != nil {
		return err
	}
	fc.f = nil
	// Then, remove file so that the next acquirer can run
	if fc.mode == np.OWRITE {
		err := fc.Remove(fc.fenceName)
		if err != nil {
			log.Printf("%v: Remove %v err %v", db.GetName(), fc.fenceName, err)
			return err
		}
	}
	return nil
}

// Update the fence file
func (fc *FenceClnt) SetFenceFile(b []byte) error {
	_, err := fc.SetFile(fc.fenceName, b)
	if err != nil {
		log.Printf("%v: SetFenceFile %v err %v", db.GetName(), fc.fenceName, err)
		return err
	}
	return fc.registerFence(0)
}

func (fc *FenceClnt) MakeFenceFileFrom(from string) error {
	err := fc.Rename(from, fc.fenceName)
	if err != nil {
		log.Printf("%v: Rename %v to %v err %v", db.GetName(), from, fc.fenceName, err)
		return err
	}
	return fc.registerFence(0)
}

func (fc *FenceClnt) AcquireFenceR() ([]byte, error) {
	ch := make(chan bool)
	for {
		// log.Printf("%v: file watch %v\n", db.GetName(), fc.fenceName)
		b, err := fc.ReadFileWatch(fc.fenceName, func(string, error) {
			ch <- true
		})
		if err != nil && strings.HasPrefix(err.Error(), "file not found") {
			// log.Printf("%v: file watch wait %v\n", db.GetName(), fc.fenceName)
			<-ch
		} else if err != nil {
			return nil, err
		} else {
			// log.Printf("%v: file watch return %v\n", db.GetName(), fc.fenceName)
			return b, fc.registerFence(np.OREAD)
		}
	}
}

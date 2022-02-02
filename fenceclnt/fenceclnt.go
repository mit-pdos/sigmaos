package fenceclnt

import (
	"encoding/json"
	"log"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

//
// Support for fences, which consists of fenceid and sequence number.
// A proc can fence all servers it has an open session with by
// registering the fence on each session.  When receiving an
// operation, the servers check if the registered fence is still valid
// by checking that the seqno in equal or larger than the last seen
// fence for that fenceid on any session. If valid the server applies
// the op, otherwise, it returns stale fence.  The key problem that
// fence avoid is an old primary that writes to a file after a new
// primary has taken over.  The old primary's write will fail because
// its fence has a lower sequence number than the new primary's fence.
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
// invalidated by a proc that has acquired the write fence for the
// configuration file by writing to the file, which increases the
// sequence number of the fence at the server holding the file.  The
// writer can then ask for a new fence from the server and reregister
// it on all its session.  Operations by readers will subsequently
// fail with stale fence error, signaling to readers that they should
// reread the configuration file.
//

type FenceClnt struct {
	fenceName string // pathname for the fence file
	*fslib.FsLib
	perm    np.Tperm
	mode    np.Tmode
	f       *np.Tfence
	lastSeq np.Tseqno
}

func MakeFenceClnt(fsl *fslib.FsLib, name string, perm np.Tperm) *FenceClnt {
	fc := &FenceClnt{}
	fc.fenceName = name
	fc.FsLib = fsl
	fc.perm = perm
	return fc
}

func (fc *FenceClnt) IsFenced() *np.Tfence {
	return fc.f
}

func (fc *FenceClnt) Name() string {
	return fc.fenceName
}

// XXX register/update may fail because another client has seen a more
// recent seqno, which the server may have not told us about because
// it lost that info due to it crashing. in that case, tell the server
// to use that more recent seqno (if the fence was acquired in write
// mode, which means this client is the current fence holder).
func (fc *FenceClnt) registerFence(mode np.Tmode) error {
	fence, err := fc.MakeFence(fc.fenceName, mode)
	if err != nil {
		log.Printf("%v: MakeFence %v err %v", db.GetName(), fc.fenceName, err)
		return err
	}
	log.Printf("%v: MakeFence %v fence %v", db.GetName(), fc.fenceName, fence)
	if fc.lastSeq > fence.Seqno {
		log.Fatalf("%v: FATAL MakeFence bad fence %v last seq %v\n", db.GetName(), fence, fc.lastSeq)
	}
	if fc.f == nil {
		fc.mode = mode
		err = fc.RegisterFence(fence)
	} else {
		// The fence holder has updated the file associated
		// with the fence; all servers about the new fence.
		err = fc.UpdateFence(fence)
	}
	if err != nil {
		log.Printf("%v: registerFence %v err %v", db.GetName(), fc.fenceName, err)
		return err
	}
	fc.lastSeq = fence.Seqno
	fc.f = &fence
	return nil
}

// Acquire a write fence, which may block, and initialize the file
// with b.  Tell servers to fence our operations.
//
// XXX cleanup on failure XXX create and write atomic
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

// Acquire a read fence, which may block until a writer has created
// the file.  Tell servers to fence our operations.
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

func (fc *FenceClnt) ReleaseFence() error {
	// First deregister fence
	if fc.f == nil {
		log.Fatalf("%v: FATAL ReleaseFence %v\n", db.GetName(), fc.fenceName)
	}
	err := fc.DeregisterFence(*fc.f)
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

//
// A few writer operations that a fence writer can perform. They will
// increase the fence's seqno, and registerFence will update servers
// to use the new fence.
//

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

//
// Convenience function
//

func (fc *FenceClnt) AcquireConfig(v interface{}) error {
	//log.Printf("%v: start AcquireConfig %v\n", db.GetName(), fc.Name())
	b, err := fc.AcquireFenceR()
	if err != nil {
		log.Printf("%v: AcquireConfig %v err %v\n", db.GetName(), fc.Name(), err)
		return err
	}
	err = json.Unmarshal(b, v)
	if err != nil {
		return err
	}
	//log.Printf("%v: AcquireConfig %v %v\n", db.GetName(), fc.Name(), v)
	return nil
}

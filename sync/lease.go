package sync

import (
	"log"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

//
// Support for leases, which are represented as regular files. A proc
// can register a lease (with a particular Qid) with all servers it
// has an open session with.  When receiving an operation, the servers
// check if the lease is still valid (by checking the Qid of the
// file).  If the Qid is unchanged with the registered lease, they
// allow the operation; otherwise, they reject the operation.
//
// There are two types of leases: write leases and read leases.  Write
// leases are for a coordinator to obtain an exclusive lease.  The
// write lease maybe invalidated anytime, for example, by a network
// partition.
//
// Multiple procs may have a read lease on, for example, a
// configuration file.  A read lease maybe invalidated by a proc that
// modifies the configuration file, signaling to the reader they
// should reread the configuration file. Operations in flight to any
// server will be rejected by those servers because the read lease is
// invalid.
//

type Lease struct {
	leaseName string // pathname for the lease file
	*fslib.FsLib
}

func MakeLease(fsl *fslib.FsLib, lName string) *Lease {
	l := &Lease{}
	l.leaseName = lName
	l.FsLib = fsl
	return l
}

//
// Write leases
//

// Wait to obtain a write lease
// XXX cleanup on failure
func (l *Lease) WaitWLease() error {
	err := l.MakeFile(l.leaseName, 0777|np.DMTMP, np.OWRITE|np.OWATCH, []byte{})
	// Sometimes we get "EOF" on shutdown
	if err != nil && err.Error() == "EOF" {
		db.DLPrintf("DLOCK", "Makefile %v err %v", l.leaseName, err)
		return err
	}
	if err != nil {
		log.Printf("%v: Makefile %v err %v", db.GetName(), l.leaseName, err)
		return err
	}
	st, err := l.Stat(l.leaseName)
	if err != nil {
		log.Printf("%v: Stat %v err %v", db.GetName(), st, err)
		return err
	}
	err = l.RegisterLock(l.leaseName, st.Qid)
	if err != nil {
		log.Printf("%v: RegisterLock %v err %v", db.GetName(), l.leaseName, err)
		return err
	}
	return nil
}

func (l *Lease) ReleaseWLease() error {
	defer l.ReleaseRLease()
	err := l.Remove(l.leaseName)
	if err != nil {
		if err.Error() == "EOF" {
			db.DLPrintf("DLOCK", "%v: Remove %v err %v", db.GetName(), l.leaseName, err)
			return err
		}
	}
	return nil
}

//
// Read leases
//

// Make the lease file
func (l *Lease) MakeLeaseFile(b []byte) error {
	err := l.MakeFile(l.leaseName, 0777|np.DMTMP, np.OWRITE, b)
	// Sometimes we get "EOF" on shutdown
	if err != nil && err.Error() == "EOF" {
		db.DLPrintf("DLOCK", "Makefile %v err %v", l.leaseName, err)
		return err
	}
	if err != nil {
		log.Printf("%v: RegisterLock %v err %v", db.GetName(), l.leaseName, err)
		return err
	}
	return nil
}

func (l *Lease) registerRLease() error {
	st, err := l.Stat(l.leaseName)
	if err != nil {
		// log.Printf("%v: Stat %v err %v", db.GetName(), st, err)
		return err
	}
	err = l.RegisterLock(l.leaseName, st.Qid)
	if err != nil {
		log.Printf("%v: RegisterLock %v err %v", db.GetName(), l.leaseName, err)
		return err
	}
	return nil
}

func (l *Lease) WaitRLease() ([]byte, error) {
	ch := make(chan bool)
	for {
		b, err := l.ReadFileWatch("name/config", func(string, error) {
			ch <- true
		})
		if err != nil {
			<-ch
		} else {
			return b, l.registerRLease()
		}
	}
}

func (l *Lease) ReleaseRLease() error {
	err := l.DeregisterLock(l.leaseName)
	if err != nil {
		log.Printf("%v: DeregisterLock %v err %v", db.GetName(), l.leaseName, err)
		return err
	}
	return nil
}

// Invalidate a lease by remove the lease file
func (l *Lease) Invalidate() error {
	err := l.Remove(l.leaseName)
	if err != nil {
		log.Printf("%v: Remove %v err %v", db.GetName(), l.leaseName, err)
		return err
	}
	return nil
}

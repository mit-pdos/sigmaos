package sync

import (
	"encoding/json"
	"fmt"
	"log"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/lease"
	np "ulambda/ninep"
)

//
// Support for leases, which consists of pathname and a Qid of that
// pathname. A proc can register a lease with all servers it has an
// open session with.  When receiving an operation, the servers check
// if the lease is still valid (by checking the Qid of the file in the
// lease).  If the Qid is unchanged from the registered lease, they
// allow the operation; otherwise, they reject the operation.
//
// Procs uses LeasePath to interact with leases, which they can use in
// two in two ways: write leases and read leases.  Write leases are
// for, for example, coordinators to obtain an exclusive LeasePath so
// that only one coorditor is active.  The write lease maybe
// invalidated anytime, for example, by a network partition, which
// allows another a new coordinator to get the LeasePath.  The old
// coordinator won't be able to perform operations at any server,
// because its lease will invalid as soon as the new coordinator
// obtains the write lease.
//
// Multiple procs may have a read lease on, for example, a LeasePath
// that represents a configuration file.  A read lease maybe
// invalidated by a proc that modifies the configuration file,
// signaling to the reader they should reread the configuration
// file. Operations in flight to any server will be rejected by those
// servers because the read lease is invalid.
//

type LeasePath struct {
	leaseName string // pathname for the lease file
	*fslib.FsLib
	perm np.Tperm
}

func MakeLeasePath(fsl *fslib.FsLib, lName string, perm np.Tperm) *LeasePath {
	l := &LeasePath{}
	l.leaseName = lName
	l.FsLib = fsl
	l.perm = perm
	return l
}

//
// Write leases
//

// Wait to obtain a write lease
// XXX cleanup on failure
// XXX create and write atomic
func (l *LeasePath) WaitWLease(b []byte) error {
	fd, err := l.Create(l.leaseName, l.perm|np.DMTMP, np.OWRITE|np.OWATCH)
	if err != nil {
		log.Printf("%v: Makefile %v err %v", db.GetName(), l.leaseName, err)
		return err
	}
	_, err = l.Write(fd, b)
	if err != nil {
		log.Printf("%v: write %v err %v", db.GetName(), l.leaseName, err)
		return err
	}
	l.Close(fd)

	st, err := l.Stat(l.leaseName)
	if err != nil {
		log.Printf("%v: Stat %v err %v", db.GetName(), st, err)
		return err
	}
	err = l.RegisterLease(lease.MakeLease(np.Split(l.leaseName), st.Qid))
	if err != nil {
		log.Printf("%v: RegisterLease %v err %v", db.GetName(), l.leaseName, err)
		return err
	}
	return nil
}

func (l *LeasePath) ReleaseWLease() error {
	err := l.ReleaseRLease()
	if err != nil {
		return err
	}
	err = l.Remove(l.leaseName)
	if err != nil {
		log.Printf("%v: Remove %v err %v", db.GetName(), l.leaseName, err)
		return err
	}
	return nil
}

//
// Read leases
//

// Make the lease file
func (l *LeasePath) MakeLeaseFile(b []byte) error {
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

func (l *LeasePath) MakeLeaseFileJson(i interface{}) error {
	b, err := json.Marshal(i)
	if err != nil {
		return fmt.Errorf("Marshal error %v", err)
	}
	return l.MakeLeaseFile(b)
}

// Make the lease file
func (l *LeasePath) MakeLeaseFileFrom(from string) error {
	err := l.Rename(from, l.leaseName)
	if err != nil {
		log.Printf("%v: Rename %v to %v err %v", db.GetName(), from, l.leaseName, err)
		return err
	}
	return nil
}

func (l *LeasePath) registerRLease() error {
	st, err := l.Stat(l.leaseName)
	if err != nil {
		// log.Printf("%v: Stat %v err %v", db.GetName(), st, err)
		return err
	}
	err = l.RegisterLease(lease.MakeLease(np.Split(l.leaseName), st.Qid))
	if err != nil {
		log.Printf("%v: RegisterLock %v err %v", db.GetName(), l.leaseName, err)
		return err
	}
	return nil
}

func (l *LeasePath) WaitRLease() ([]byte, error) {
	ch := make(chan bool)
	for {
		b, err := l.ReadFileWatch(l.leaseName, func(string, error) {
			ch <- true
		})
		if err != nil {
			<-ch
		} else {
			return b, l.registerRLease()
		}
	}
}

func (l *LeasePath) ReleaseRLease() error {
	err := l.DeregisterLease(l.leaseName)
	if err != nil {
		log.Printf("%v: DeregisterLease %v err %v", db.GetName(), l.leaseName, err)
		return err
	}
	return nil
}

// Invalidate a lease by remove the lease file
func (l *LeasePath) Invalidate() error {
	err := l.Remove(l.leaseName)
	if err != nil {
		log.Printf("%v: Remove %v err %v", db.GetName(), l.leaseName, err)
		return err
	}
	return nil
}

// Invalidate a lease by remove the lease file
func (l *LeasePath) RenameTo(to string) error {
	err := l.Rename(l.leaseName, to)
	if err != nil {
		log.Printf("%v: Rename %v to %v err %v", db.GetName(), l.leaseName, to, err)
		return err
	}
	return nil
}

package mr

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/klauspost/readahead"

	"ulambda/awriter"
	"ulambda/crash"
	db "ulambda/debug"
	"ulambda/delay"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/rand"
	"ulambda/writer"
)

type MapT func(string, io.Reader, func(*KeyValue) error) error

type wrt struct {
	wrt  *writer.Writer
	awrt *awriter.Writer
	bwrt *bufio.Writer
}

type Mapper struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	mapf        MapT
	nreducetask int
	input       string
	file        string
	wrts        []*wrt
	rand        string
}

// XXX create in a temporary file and then rename
func makeMapper(mapf MapT, args []string) (*Mapper, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("MakeMapper: too few arguments %v", args)
	}
	m := &Mapper{}
	m.mapf = mapf
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("MakeMapper: nreducetask %v isn't int", args[0])
	}
	m.nreducetask = n
	m.input = args[1]
	m.file = path.Base(m.input)
	m.rand = rand.String(16)
	m.wrts = make([]*wrt, m.nreducetask)

	m.FsLib = fslib.MakeFsLib("mapper-" + proc.GetPid().String() + " " + m.input)
	m.ProcClnt = procclnt.MakeProcClnt(m.FsLib)
	m.Started()

	crash.Crasher(m.FsLib)
	delay.SetDelayRPC(3)
	return m, nil
}

func (m *Mapper) initMapper() error {
	// Make a directory for holding the output files of a map task.  Ignore
	// error in case it already exits.  XXX who cleans up?
	d := np.UX + "/~ip/m-" + m.file
	m.MkDir(d, 0777)

	// Create the output files
	for r := 0; r < m.nreducetask; r++ {
		// create temp output file
		oname := np.UX + "/~ip/m-" + m.file + "/r-" + strconv.Itoa(r) + m.rand
		w, err := m.CreateWriter(oname, 0777, np.OWRITE)
		if err != nil {
			return fmt.Errorf("%v: create %v err %v\n", proc.GetName(), oname, err)
		}
		aw := awriter.NewWriterSize(w, BUFSZ)
		bw := bufio.NewWriterSize(aw, BUFSZ)
		m.wrts[r] = &wrt{w, aw, bw}
	}
	return nil
}

// XXX use writercloser
func (m *Mapper) closewrts() error {
	for r := 0; r < m.nreducetask; r++ {
		if err := m.wrts[r].bwrt.Flush(); err != nil {
			return fmt.Errorf("%v: flush %v err %v\n", proc.GetName(), m.wrts[r], err)
		}
		if err := m.wrts[r].awrt.Close(); err != nil {
			return fmt.Errorf("%v: aclose %v err %v\n", proc.GetName(), m.wrts[r], err)
		}
		if err := m.wrts[r].wrt.Close(); err != nil {
			return fmt.Errorf("%v: close %v err %v\n", proc.GetName(), m.wrts[r], err)
		}
	}
	return nil
}

// Inform reducer where to find map output
func (m *Mapper) informReducer() error {
	st, err := m.Stat(np.UX + "/~ip")
	if err != nil {
		return fmt.Errorf("%v: stat %v err %v\n", proc.GetName(), np.UX+"/~ip", err)
	}
	for r := 0; r < m.nreducetask; r++ {
		fn := np.UX + "/~ip/m-" + m.file + "/r-" + strconv.Itoa(r)
		err = m.Rename(fn+m.rand, fn)
		if err != nil {
			return fmt.Errorf("%v: rename %v -> %v err %v\n", proc.GetName(),
				fn+m.rand, fn, err)
		}

		name := "name/mr/r/" + strconv.Itoa(r) + "/m-" + m.file

		// Remove name in case an earlier mapper created the
		// symlink.  A reducer will have opened and is reading
		// the old target, open the new input file and read
		// the new target, or file because there is no
		// symlink. Failing is fine because the coodinator
		// will start a new reducer once this map completes.
		// We could use rename to atomically remove and create
		// the symlink if we want to avoid the failing case.
		m.Remove(name)

		target := np.UX + "/" + st.Name + "/m-" + m.file + "/r-" + strconv.Itoa(r) + "/"
		err = m.Symlink([]byte(target), name, 0777)
		if err != nil {
			if np.IsErrNotfound(err) {
				// If the reducer successfully completed, the reducer dir won't be found.
				// In that case, we don't want to mark the mapper as "failed", since this
				// will loop infinitely.
				log.Printf("%v: symlink %v err %v\n", proc.GetName(), name, err)
			}
			db.DFatalf("%v: FATA/L symlink %v err %v\n", proc.GetName(), name, err)
		}
	}
	return nil
}

func (m *Mapper) emit(kv *KeyValue) error {
	r := Khash(kv.Key) % m.nreducetask
	if err := fslib.WriteJsonRecord(m.wrts[r].bwrt, kv); err != nil {
		return fmt.Errorf("%v: mapper %v err %v", proc.GetName(), r, err)
	}
	return nil
}

func (m *Mapper) doMap() error {
	start := time.Now()
	rdr, err := m.OpenReader(m.input)
	if err != nil {
		db.DFatalf("%v: read %v err %v", proc.GetName(), m.input, err)
	}

	db.DPrintf("MR0", "Open %v\n", time.Since(start).Milliseconds())
	start = time.Now()

	//brdr := bufio.NewReaderSize(rdr, BUFSZ)
	ardr, err := readahead.NewReaderSize(rdr, 4, BUFSZ)
	if err != nil {
		db.DFatalf("%v: readahead.NewReaderSize err %v", proc.GetName(), err)
	}
	if err := m.mapf(m.input, ardr, m.emit); err != nil {
		return err
	}
	db.DPrintf("MR0", "Mapf %v\n", time.Since(start).Milliseconds())
	start = time.Now()

	if err := m.closewrts(); err != nil {
		return err
	}
	db.DPrintf("MR0", "Close %v\n", time.Since(start).Milliseconds())
	start = time.Now()

	if err := m.informReducer(); err != nil {
		return err
	}
	db.DPrintf("MR0", "Inform %v\n", time.Since(start).Milliseconds())
	start = time.Now()

	return nil
}

func RunMapper(mapf MapT, args []string) {
	m, err := makeMapper(mapf, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	err = m.initMapper()
	if err != nil {
		m.Exited(proc.MakeStatusErr(err.Error(), nil))
		return
	}
	err = m.doMap()
	if err == nil {
		m.Exited(proc.MakeStatus(proc.StatusOK))
	} else {
		m.Exited(proc.MakeStatusErr(err.Error(), nil))
	}
}

package mr

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	// "github.com/klauspost/readahead"

	"ulambda/awriter"
	"ulambda/crash"
	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/rand"
	"ulambda/test"
	"ulambda/writer"
)

type wrt struct {
	wrt  *writer.Writer
	awrt *awriter.Writer
	bwrt *bufio.Writer
}

type Mapper struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	mapf        MapT
	job         string
	nreducetask int
	input       string
	bin         string
	wrts        []*wrt
	rand        string
}

func makeMapper(mapf MapT, args []string) (*Mapper, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("MakeMapper: too few arguments %v", args)
	}
	m := &Mapper{}
	m.mapf = mapf
	m.job = args[0]
	n, err := strconv.Atoi(args[1])
	if err != nil {
		return nil, fmt.Errorf("MakeMapper: nreducetask %v isn't int", args[1])
	}
	m.nreducetask = n
	m.input = args[2]
	m.bin = path.Base(m.input)
	m.rand = rand.String(16)
	m.wrts = make([]*wrt, m.nreducetask)

	m.FsLib = fslib.MakeFsLib("mapper-" + proc.GetPid().String() + " " + m.input)
	m.ProcClnt = procclnt.MakeProcClnt(m.FsLib)
	if err := m.Started(); err != nil {
		return nil, fmt.Errorf("MakeMapper couldn't start %v", args)
	}
	crash.Crasher(m.FsLib)
	return m, nil
}

func (m *Mapper) initMapper() error {

	// Make a directory for holding the output files of a map task.  Ignore
	// error in case it already exits.  XXX who cleans up?
	m.MkDir(MLOCALDIR, 0777)
	m.MkDir(LocalOut(m.job), 0777)
	m.MkDir(Moutdir(m.job, m.bin), 0777)

	// Create the output files
	for r := 0; r < m.nreducetask; r++ {
		// create temp output shard for reducer r
		oname := mshardfile(m.job, m.bin, r) + m.rand
		w, err := m.CreateWriter(oname, 0777, np.OWRITE)
		if err != nil {
			m.closewrts()
			return fmt.Errorf("%v: create %v err %v\n", proc.GetName(), oname, err)
		}
		aw := awriter.NewWriterSize(w, test.BUFSZ)
		bw := bufio.NewWriterSize(aw, test.BUFSZ)
		m.wrts[r] = &wrt{w, aw, bw}
	}
	return nil
}

// XXX use writercloser
func (m *Mapper) closewrts() (np.Tlength, error) {
	n := np.Tlength(0)
	for r := 0; r < m.nreducetask; r++ {
		if m.wrts[r] != nil {
			if err := m.wrts[r].awrt.Close(); err != nil {
				return 0, fmt.Errorf("%v: aclose %v err %v\n", proc.GetName(), m.wrts[r], err)
			}
			if err := m.wrts[r].wrt.Close(); err != nil {
				return 0, fmt.Errorf("%v: close %v err %v\n", proc.GetName(), m.wrts[r], err)
			}
			n += m.wrts[r].wrt.Nbytes()

		}
	}
	return n, nil
}

func (m *Mapper) flushwrts() error {
	for r := 0; r < m.nreducetask; r++ {
		if err := m.wrts[r].bwrt.Flush(); err != nil {
			return fmt.Errorf("%v: flush %v err %v\n", proc.GetName(), m.wrts[r], err)
		}
	}
	return nil
}

// Inform reducer where to find map output
func (m *Mapper) informReducer() error {
	st, err := m.Stat(MLOCALSRV)
	if err != nil {
		return fmt.Errorf("%v: stat %v err %v\n", proc.GetName(), MLOCALSRV, err)
	}
	for r := 0; r < m.nreducetask; r++ {
		fn := mshardfile(m.job, m.bin, r)
		err = m.Rename(fn+m.rand, fn)
		if err != nil {
			return fmt.Errorf("%v: rename %v -> %v err %v\n", proc.GetName(), fn+m.rand, fn, err)
		}

		name := symname(m.job, strconv.Itoa(r), m.bin)

		// Remove name in case an earlier mapper created the
		// symlink.  A reducer may have opened and is reading
		// the old target, open the new input file and read
		// the new target, or fail because there is no
		// symlink. Failing is fine because the coodinator
		// will start a new reducer once this map completes.
		// We could use rename to atomically remove and create
		// the symlink if we want to avoid the failing case.
		m.Remove(name)

		target := shardtarget(m.job, st.Name, m.bin, r)

		db.DPrintf("MR", "name %s target %s\n", name, target)

		err = m.Symlink([]byte(target), name, 0777)
		if err != nil {
			db.DFatalf("%v: FATAL symlink %v err %v\n", proc.GetName(), name, err)
		}
	}
	return nil
}

func (m *Mapper) emit(kv *KeyValue) error {
	r := Khash(kv.K) % m.nreducetask
	if err := fslib.WriteJsonRecord(m.wrts[r].bwrt, kv); err != nil {
		return fmt.Errorf("%v: mapper %v err %v", proc.GetName(), r, err)
	}
	return nil
}

func (m *Mapper) doSplit(s *Split) (np.Tlength, error) {
	rdr, err := m.OpenReader(s.File)
	if err != nil {
		db.DFatalf("%v: read %v err %v", proc.GetName(), s.File, err)
	}
	defer rdr.Close()
	rdr.Lseek(s.Offset)

	brdr := bufio.NewReaderSize(rdr, test.BUFSZ)
	scanner := bufio.NewScanner(brdr)

	// advance scanner to new line after start, if start != 0
	n := 0
	if s.Offset != 0 {
		scanner.Scan()
		l := scanner.Text()
		n += len(l) + 1 // 1 for newline
	}
	for scanner.Scan() {
		l := scanner.Text()
		n += len(l) + 1 // 1 for newline
		if err := m.mapf(m.input, strings.NewReader(l), m.emit); err != nil {
			return 0, err
		}

		if np.Tlength(n) > s.Length {
			break
		}
	}
	return np.Tlength(n), nil
}

func (m *Mapper) doMap() (np.Tlength, np.Tlength, error) {
	rdr, err := m.OpenReader(m.input)
	if err != nil {
		return 0, 0, err
	}
	dec := json.NewDecoder(rdr)
	ni := np.Tlength(0)
	for {
		var s Split
		if err := dec.Decode(&s); err == io.EOF {
			break
		} else if err != nil {
			c, _ := m.GetFile(m.input)
			db.DPrintf("MR", "Mapper %s: decode %v err %v\n", m.bin, string(c), err)
			return 0, 0, err
		}
		db.DPrintf("MR", "Mapper %s: process split %v\n", m.bin, s)
		n, err := m.doSplit(&s)
		if err != nil {
			return 0, 0, err
		}
		ni += n
	}
	if err := m.flushwrts(); err != nil {
		return 0, 0, err
	}
	nout, err := m.closewrts()
	if err != nil {
		return 0, 0, err
	}
	if err := m.informReducer(); err != nil {
		return 0, 0, err
	}
	return ni, nout, nil
}

func RunMapper(mapf MapT, args []string) {
	m, err := makeMapper(mapf, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	if err = m.initMapper(); err != nil {
		m.Exited(proc.MakeStatusErr(err.Error(), nil))
		return
	}
	start := time.Now()
	nin, nout, err := m.doMap()
	if err == nil {
		m.Exited(proc.MakeStatusInfo(proc.StatusOK, m.input,
			Result{true, m.input, nin, nout, time.Since(start).Milliseconds()}))
	} else {
		m.Exited(proc.MakeStatusErr(err.Error(), nil))
	}
}

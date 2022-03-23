package mr

import (
	"fmt"
	"log"
	"os"
	"path"
	"strconv"

	"ulambda/crash"
	"ulambda/delay"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/rand"
	"ulambda/writer"
)

type MapT func(string, string) []KeyValue

type Mapper struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	mapf        MapT
	nreducetask int
	input       string
	file        string
	fds         []*writer.Writer
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
	m.fds = make([]*writer.Writer, m.nreducetask)

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
	var err error
	for r := 0; r < m.nreducetask; r++ {
		// create temp output file
		oname := np.UX + "/~ip/m-" + m.file + "/r-" + strconv.Itoa(r) + m.rand
		m.fds[r], err = m.CreateWriter(oname, 0777, np.OWRITE)
		if err != nil {
			return fmt.Errorf("%v: create %v err %v\n", proc.GetName(), oname, err)
		}
	}
	return nil
}

func (m *Mapper) closefds() error {
	for r := 0; r < m.nreducetask; r++ {
		err := m.fds[r].Close()
		if err != nil {
			return fmt.Errorf("%v: close %v err %v\n", proc.GetName(), m.fds[r], err)
		}
	}
	return nil
}

func (m *Mapper) mapper(txt string) error {
	kvs := m.mapf(m.input, txt)

	// log.Printf("%v: Map %v: kvs = %v\n", proc.GetName(), m.input, kvs)

	// split
	skvs := make([][]KeyValue, m.nreducetask)
	for _, kv := range kvs {
		r := Khash(kv.Key) % m.nreducetask
		skvs[r] = append(skvs[r], kv)
	}
	for r := 0; r < m.nreducetask; r++ {
		if err := m.fds[r].WriteJsonRecord(skvs[r]); err != nil {
			return fmt.Errorf("%v: mapper %v err %v", proc.GetName(), r, err)
		}
	}
	return nil
}

func (m *Mapper) doMap() error {
	b, err := m.GetFile(m.input)
	if err != nil {
		log.Fatalf("%v: read %v err %v", proc.GetName(), m.input, err)
	}
	txt := string(b)
	err = m.mapper(txt)
	if err != nil {
		return err
	}

	// Inform reducer where to find map output
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
			log.Fatalf("%v: FATA/L symlink %v err %v\n", proc.GetName(), name, err)
		}
	}
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

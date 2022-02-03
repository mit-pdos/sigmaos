package mr

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"

	"ulambda/crash"
	db "ulambda/debug"
	"ulambda/delay"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
	"ulambda/rand"
)

type MapT func(string, string) []KeyValue

type Mapper struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	mapf        MapT
	nreducetask int
	input       string
	file        string
	fds         []int
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
	m.fds = make([]int, m.nreducetask)

	m.FsLib = fslib.MakeFsLib("mapper-" + m.input)
	m.ProcClnt = procclnt.MakeProcClnt(m.FsLib)

	m.Started(proc.GetPid())

	crash.Crasher(m.FsLib)
	delay.SetDelayRPC(100)
	return m, nil
}

func (m *Mapper) initMapper() error {
	// Make a directory for holding the output files of a map task.  Ignore
	// error in case it already exits.  XXX who cleans up?
	d := "name/ux/~ip/m-" + m.file
	m.Mkdir(d, 0777)

	// Create the output files
	var err error
	for r := 0; r < m.nreducetask; r++ {
		// create temp output file
		oname := "name/ux/~ip/m-" + m.file + "/r-" + strconv.Itoa(r) + m.rand
		m.fds[r], err = m.CreateFile(oname, 0777, np.OWRITE)
		if err != nil {
			return fmt.Errorf("%v: create %v err %v\n", db.GetName(), oname, err)
		}
	}
	return nil
}

func (m *Mapper) closefds() error {
	for r := 0; r < m.nreducetask; r++ {
		err := m.Close(m.fds[r])
		if err != nil {
			return fmt.Errorf("%v: close %v err %v\n", db.GetName(), m.fds[r], err)
		}
	}
	return nil
}

func (m *Mapper) mapper(txt string) error {
	kvs := m.mapf(m.input, txt)

	// log.Printf("%v: Map %v: kvs = %v\n", db.GetName(), m.input, kvs)

	// split
	skvs := make([][]KeyValue, m.nreducetask)
	for _, kv := range kvs {
		r := Khash(kv.Key) % m.nreducetask
		skvs[r] = append(skvs[r], kv)
	}

	for r := 0; r < m.nreducetask; r++ {
		b, err := json.Marshal(skvs[r])
		if err != nil {
			fmt.Errorf("%v: marshal error %v", db.GetName(), err)
		}
		lbuf := make([]byte, binary.MaxVarintLen64)
		binary.PutVarint(lbuf, int64(len(b)))
		_, err = m.Write(m.fds[r], lbuf)
		if err != nil {
			// maybe another worker finished earlier
			// XXX handle partial writing of intermediate files
			return fmt.Errorf("doMap write error %v %v\n", r, err)
		}
		_, err = m.Write(m.fds[r], b)
		if err != nil {
			return fmt.Errorf("%v: write %v err %v\n", db.GetName(), r, err)
		}
	}
	return nil
}

func (m *Mapper) doMap() error {
	b, err := m.ReadFile(m.input)
	if err != nil {
		log.Fatalf("%v: read %v err %v", db.GetName(), m.input, err)
	}
	txt := string(b)
	err = m.mapper(txt)
	if err != nil {
		return err
	}

	// Inform reducer where to find map output
	st, err := m.Stat("name/ux/~ip")
	if err != nil {
		return fmt.Errorf("%v: stat %v err %v\n", db.GetName(), "name/ux/~ip", err)
	}
	for r := 0; r < m.nreducetask; r++ {
		fn := "name/ux/~ip/m-" + m.file + "/r-" + strconv.Itoa(r)
		err = m.Rename(fn+m.rand, fn)
		if err != nil {
			return fmt.Errorf("%v: rename %v -> %v err %v\n", db.GetName(),
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

		target := "name/ux/" + st.Name + "/m-" + m.file + "/r-" + strconv.Itoa(r) + "/"
		err = m.Symlink([]byte(target), name, 0777)
		if err != nil {
			// If the reducer successfully completed, the reducer dir won't be found.
			// In that case, we don't want to mark the mapper as "failed", since this
			// will loop infinitely.
			log.Printf("%v: symlink %v err %v\n", db.GetName(), name, err)
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
		m.Exited(proc.GetPid(), err.Error())
		return
	}
	err = m.doMap()
	if err == nil {
		m.Exited(proc.GetPid(), "OK")
	} else {
		m.Exited(proc.GetPid(), err.Error())
	}
}

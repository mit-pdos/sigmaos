package mr

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path"
	"strconv"

	// db "ulambda/debug"
	"ulambda/crash"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procinit"
)

type MapT func(string, string) []KeyValue

type Mapper struct {
	*fslib.FsLib
	proc.ProcClnt
	mapf        MapT
	crash       string
	nreducetask int
	input       string
	file        string
	fds         []int
	rand        string
	//	fd     int
}

// XXX create in a temporary file and then rename
func MakeMapper(mapf MapT, args []string) (*Mapper, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("MakeMapper: too few arguments %v", args)
	}
	m := &Mapper{}
	m.mapf = mapf
	m.crash = args[0]
	n, err := strconv.Atoi(args[1])
	if err != nil {
		return nil, fmt.Errorf("MakeMapper: nreducetask %v isn't int", args[0])
	}
	m.nreducetask = n
	m.input = args[2]
	m.file = path.Base(m.input)
	m.rand = strconv.Itoa(rand.Intn(100000))
	m.fds = make([]int, m.nreducetask)

	m.FsLib = fslib.MakeFsLib("mapper-" + m.input)
	log.Printf("MakeMapper %v\n", args)
	m.ProcClnt = procinit.MakeProcClnt(m.FsLib, procinit.GetProcLayersMap())

	// Make a directory for holding the output files of a map task.  Ignore
	// error in case it already exits.  XXX who cleans up?
	d := "name/ux/~ip/m-" + m.file
	m.Mkdir(d, 0777)

	//  when using fsreader:
	//
	//      m.fd, err = m.Open(m.input, np.OREAD)
	//      if err != nil {
	//              return nil, fmt.Errorf("Makemapper: unknown %v\n", m.input)
	//      }

	// Create the output files
	for r := 0; r < m.nreducetask; r++ {
		// create temp output file
		oname := "name/ux/~ip/m-" + m.file + "/r-" + strconv.Itoa(r) + m.rand
		m.fds[r], err = m.CreateFile(oname, 0777, np.OWRITE)
		if err != nil {
			return nil, fmt.Errorf("MakeMapper: cannot create %v err %v\n", oname, err)
		}
	}
	m.Started(proc.GetPid())

	if m.crash == "YES" {
		crash.Crasher(m.FsLib)
	}

	return m, nil
}

func (m *Mapper) Map(txt string) {
	kvs := m.mapf(m.input, txt)
	// log.Printf("Map %v: kvs = %v\n", m.input, kvs)

	// split
	skvs := make([][]KeyValue, m.nreducetask)
	for _, kv := range kvs {
		r := Khash(kv.Key) % m.nreducetask
		skvs[r] = append(skvs[r], kv)
	}

	for r := 0; r < m.nreducetask; r++ {
		b, err := json.Marshal(skvs[r])
		if err != nil {
			log.Fatal("doMap marshal error", err)
		}
		lbuf := make([]byte, binary.MaxVarintLen64)
		binary.PutVarint(lbuf, int64(len(b)))
		_, err = m.Write(m.fds[r], lbuf)
		if err != nil {
			// maybe another worker finished earlier
			// XXX handle partial writing of intermediate files
			log.Printf("doMap write error %v %v\n", r, err)
			return
		}
		_, err = m.Write(m.fds[r], b)
		if err != nil {
			log.Printf("doMap write error %v %v\n", r, err)
			return
		}
	}
}

func (m *Mapper) doMap() error {
	b, err := m.ReadFile(m.input)
	if err != nil {
		log.Fatalf("Error ReadFile in Mapper.doMap: %v", err)
	}
	txt := string(b)
	m.Map(txt)

	//
	//  When using fsreader:
	//
	//	rest := ""
	//	for {
	//		b, err := m.Read(m.fd, memfs.PIPESZ)
	//		if err != nil || len(b) == 0 {
	//			db.DLPrintf("MAPPER", "Read %v %v", m.input, err)
	//			m.Map(rest)
	//			break
	//		}
	//
	//		// backup up if in the middle of word
	//		txt := string(b)
	//		for i := len(txt) - 1; i >= 0; i-- {
	//			if txt[i] == ' ' {
	//				t := rest
	//				rest = txt[i+1:]
	//				txt = t + txt[0:i]
	//				break
	//			}
	//		}
	//
	//		m.Map(txt)
	//	}
	//	err := m.Close(m.fd)
	//	if err != nil {
	//		log.Printf("close failed %v\n", err)
	//	}

	if m.crash == "YES" {
		crash.MaybeDelay()
	}

	// Inform reducer where to find map output
	st, err := m.Stat("name/ux/~ip")
	if err != nil {
		return err
	}
	for r := 0; r < m.nreducetask; r++ {
		err = m.Close(m.fds[r])
		if err != nil {
			log.Printf("Close failed %v %v\n", m.fds[r], err)
			return err
		}
		fn := "name/ux/~ip/m-" + m.file + "/r-" + strconv.Itoa(r)
		err = m.Rename(fn+m.rand, fn)
		if err != nil {
			log.Fatalf("rename failed %v\n", err)
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
		err = m.Symlink(target, name, 0777)
		if err != nil {
			log.Printf("Mapper: cannot create symlink %v %v\n", name, err)
			return err
		}
	}
	return nil
}

func (m *Mapper) Work() {
	err := m.doMap()
	if err != nil {
		log.Printf("doMap failed %v\n", err)
		os.Exit(1)
	}

}

func (m *Mapper) Exit() {
	m.Exited(proc.GetPid(), "OK")
}

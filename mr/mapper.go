package mr

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procinit"
)

type MapT func(string, string) []KeyValue

type Mapper struct {
	*fslib.FsLib
	proc.ProcClnt
	mapf  MapT
	input string
	file  string
	fds   []int
	//	fd     int
}

// XXX create in a temporary file and then rename
func MakeMapper(mapf MapT, args []string) (*Mapper, error) {
	if len(args) != 1 {
		return nil, errors.New("MakeMapper: too few arguments")
	}
	m := &Mapper{}
	db.Name("mapper")
	m.mapf = mapf
	m.input = args[0]
	m.file = path.Base(m.input)
	m.fds = make([]int, NReduce)

	m.FsLib = fslib.MakeFsLib("mapper")
	log.Printf("MakeMapper %v\n", args)
	m.ProcClnt = procinit.MakeProcClnt(m.FsLib, procinit.GetProcLayersMap())

	// Make a directory for holding the output files of a map task
	d := "name/ux/~ip/m-" + m.file
	err := m.Mkdir(d, 0777)
	if err != nil {
		return nil, fmt.Errorf("MakeMapper: cannot create dir %v err %v\n", d, err)
	}

	//  when using fsreader:
	//
	//      m.fd, err = m.Open(m.input, np.OREAD)
	//      if err != nil {
	//              return nil, fmt.Errorf("Makemapper: unknown %v\n", m.input)
	//      }

	// Create the output files
	for r := 0; r < NReduce; r++ {
		oname := "name/ux/~ip/m-" + m.file + "/r-" + strconv.Itoa(r)
		m.fds[r], err = m.CreateFile(oname, 0777, np.OWRITE)
		if err != nil {
			return nil, fmt.Errorf("MakeMapper: cannot create %v err %v\n", oname, err)
		}
	}
	m.Started(procinit.GetPid())
	return m, nil
}

func (m *Mapper) Map(txt string) {
	kvs := m.mapf(m.input, txt)
	// log.Printf("Map %v: kvs = %v\n", m.input, kvs)

	// split
	skvs := make([][]KeyValue, NReduce)
	for _, kv := range kvs {
		r := Khash(kv.Key) % NReduce
		skvs[r] = append(skvs[r], kv)
	}

	for r := 0; r < NReduce; r++ {
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

	// Inform reducer where to find map output
	st, err := m.Stat("name/ux/~ip")
	if err != nil {
		return err
	}
	for r := 0; r < NReduce; r++ {
		err = m.Close(m.fds[r])
		if err != nil {
			log.Printf("Close failed %v %v\n", m.fds[r], err)
			return err
		}
		name := "name/mr/r/" + strconv.Itoa(r) + "/m-" + m.file
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
	m.Exited(procinit.GetPid(), "OK")
}

package mr

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfs"
	np "ulambda/ninep"
)

type MapT func(string, string) []KeyValue

type Mapper struct {
	*fslib.FsLib
	mapf   MapT
	pid    string
	input  string
	output string
	fd     int
	fds    []int
}

// XXX create in a temporary file and then rename
func MakeMapper(mapf MapT, args []string) (*Mapper, error) {
	if len(args) != 3 {
		return nil, errors.New("MakeMapper: too few arguments")
	}
	m := &Mapper{}
	db.Name("mapper")
	m.mapf = mapf
	m.pid = args[0]
	m.input = args[1]
	m.output = args[2]
	m.fds = make([]int, NReduce)

	m.FsLib = fslib.MakeFsLib("mapper")
	log.Printf("MakeMapper %v\n", args)

	err := m.Mkdir("name/ux/~ip/m-"+m.output, 0777)
	if err != nil {
		return nil, fmt.Errorf("Makemapper: cannot create %v: %v\n", m.output, err)
	}
	m.fd, err = m.Open(m.input, np.OREAD)
	if err != nil {
		return nil, fmt.Errorf("Makemapper: unknown %v\n", m.input)
	}
	for r := 0; r < NReduce; r++ {
		oname := "name/ux/~ip/m-" + m.output + "/r-" + strconv.Itoa(r)
		m.fds[r], err = m.CreateFile(oname, 0777, np.OWRITE)
		if err != nil {
			return nil, fmt.Errorf("Makemapper: cannot create %v\n", oname)
		}
	}
	m.Started(m.pid)
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
	rest := ""
	for {
		b, err := m.Read(m.fd, memfs.PIPESZ)
		if err != nil || len(b) == 0 {
			db.DLPrintf("MAPPER", "Read %v %v", m.input, err)
			m.Map(rest)
			break
		}

		// backup up if in the middle of word
		txt := string(b)
		for i := len(txt) - 1; i >= 0; i-- {
			if txt[i] == ' ' {
				t := rest
				rest = txt[i+1:]
				txt = t + txt[0:i]
				break
			}
		}

		m.Map(txt)
	}
	err := m.Close(m.fd)
	if err != nil {
		log.Printf("close failed %v\n", err)
	}

	// Inform reducer where to find map output
	st, err := m.Stat("name/ux/~ip")
	if err != nil {
		return err
	}

	for r := 0; r < NReduce; r++ {
		err = m.Close(m.fds[r])
		if err != nil {
			db.DLPrintf("MAPPER", "Close failed %v %v\n", m.fd, err)
			return err
		}
		name := "name/fs/" + strconv.Itoa(r) + "/m-" + m.output
		target := "name/ux/" + st.Name + "/m-" + m.output + "/r-" + strconv.Itoa(r) + "/"
		err = m.Symlink(target, name, 0777)
		if err != nil {
			db.DLPrintf("MAPPER", "Mapper: cannot create symlink %v %v\n", name, err)
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

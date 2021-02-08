package mr

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfs"
	np "ulambda/ninep"
)

type MapT func(string, string) []KeyValue

type Mapper struct {
	clnt   *fslib.FsLib
	mapf   MapT
	pid    string
	input  string
	output string
	fd     int
	fds    []int
}

// XXX create in a temporary file and then rename
func MakeMapper(mapf MapT, args []string) (*Mapper, error) {
	m := &Mapper{}
	m.clnt = fslib.MakeFsLib("mapper")
	m.mapf = mapf
	if len(args) != 3 {
		return nil, errors.New("MakeMapper: too few arguments")
	}
	log.Printf("MakeMapper %v\n", args)
	m.pid = args[0]
	m.input = args[1]
	m.output = args[2]
	m.fds = make([]int, NReduce)
	var err error
	m.fd, err = m.clnt.Open(m.input, np.OREAD)
	if err != nil {
		return nil, fmt.Errorf("Makemapper: unknown %v\n", m.input)
	}
	for r := 0; r < NReduce; r++ {
		oname := "name/fs/" + strconv.Itoa(r) + "/mr-" + m.output
		m.fds[r], err = m.clnt.CreateFile(oname, np.OWRITE)
		if err != nil {
			return nil, fmt.Errorf("Makemapper: cannot create %v\n", oname)
		}
	}
	m.clnt.Started(m.pid)
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
		_, err = m.clnt.Write(m.fds[r], lbuf)
		if err != nil {
			// maybe another worker finished earlier
			// XXX handle partial writing of intermediate files
			log.Printf("doMap write error %v %v\n", r, err)
			return
		}
		_, err = m.clnt.Write(m.fds[r], b)
		if err != nil {
			log.Printf("doMap write error %v %v\n", r, err)
			return
		}
	}
}

func (m *Mapper) doMap() {
	rest := ""
	for {
		b, err := m.clnt.Read(m.fd, memfs.PIPESZ)
		if err != nil || len(b) == 0 {
			db.DPrintf("Read %v %v", m.input, err)
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
	err := m.clnt.Close(m.fd)
	if err != nil {
		db.DPrintf("Close failed %v %v\n", m.fd, err)
	}
	for r := 0; r < NReduce; r++ {
		err = m.clnt.Close(m.fds[r])
		if err != nil {
			db.DPrintf("Close failed %v %v\n", m.fd, err)
		}
	}
}

func (m *Mapper) Work() {
	m.doMap()
	m.clnt.Close(m.fd)
	m.clnt.Exiting(m.pid)
}

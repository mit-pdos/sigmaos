package mr

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"

	"ulambda/fslib"
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

func MakeMapper(mapf MapT, args []string) (*Mapper, error) {
	m := &Mapper{}
	m.clnt = fslib.MakeFsLib(false)
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
		oname := "name/fs/mr-wc/" + strconv.Itoa(r) + "/mr-" + m.output
		m.fds[r], err = m.clnt.CreateFile(oname, np.OWRITE)
		if err != nil {
			return nil, fmt.Errorf("Makemapper: cannot create %v\n", oname)
		}
	}
	return m, nil
}

func (m *Mapper) doMap() {
	// XXX 8192 not a word boundary
	for {
		b, err := m.clnt.Read(m.fd, 8192)
		if err != nil {
			log.Fatalf("Read %v %v", m.input, err)
		}
		if len(b) == 0 {
			log.Printf("Mapper: reading done\n")
			break
		}
		kvs := m.mapf(m.input, string(b))
		// log.Printf("Map %v: kvs = %v\n", m.input, len(kvs))

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
			// XXX put this in make-lambda.sh
			_, err = m.clnt.Write(m.fds[r], b)
			if err != nil {
				// maybe another worker finished earlier
				// XXX handle partial writing of intermediate files
				log.Printf("doMap write error %v %v\n", r, err)
				return
			}
		}

	}
	err := m.clnt.Close(m.fd)
	if err != nil {
		log.Printf("Close failed %v %v\n", m.fd, err)
	}
	for r := 0; r < NReduce; r++ {
		err = m.clnt.Close(m.fds[r])
		if err != nil {
			log.Printf("Close failed %v %v\n", m.fd, err)
		}
	}
}

func (m *Mapper) Work() {
	m.doMap()
}

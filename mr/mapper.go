package mr

import (
	"encoding/json"
	"errors"
	"log"
	"strconv"

	"ulambda/fslib"
	// np "ulambda/ninep"
)

type MapT func(string, string) []KeyValue

type Mapper struct {
	clnt   *fslib.FsLib
	mapf   MapT
	input  string
	output string
}

func MakeMapper(mapf MapT, inputs []string) (*Mapper, error) {
	m := &Mapper{}
	m.clnt = fslib.MakeFsLib(false)
	m.mapf = mapf
	if len(inputs) != 2 {
		return nil, errors.New("MakeMapper: too few arguments")
	}
	log.Printf("MakeMapper %v\n", inputs)
	m.input = inputs[0]
	m.output = inputs[1]
	return m, nil
}

func (m *Mapper) doMap() {
	contents, err := m.clnt.ReadFile(m.input)
	if err != nil {
		log.Fatalf("readContents %v %v", m.input, err)
	}
	kvs := m.mapf(m.input, string(contents))
	log.Printf("Map %v: kvs = %v\n", m.input, len(kvs))

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
		oname := "name/fs/mr-wc/" + strconv.Itoa(r) + "/mr-" + m.output
		err = m.clnt.MakeFile(oname, b)
		if err != nil {
			// maybe another worker finished earlier
			// XXX handle partial writing of intermediate files
			log.Printf("doMap create error %v %v\n", oname, err)
			return
		}
		log.Printf("new reduce task %v %v\n", oname, len(skvs[r]))
	}

}

func (m *Mapper) Work() {
	m.doMap()
}

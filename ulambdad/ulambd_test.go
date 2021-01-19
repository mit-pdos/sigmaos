package ulambd

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"testing"
)

func TestSimple(t *testing.T) {
	a := Attr{".yyyy", []string{"xxxx", "xxx2"}, []string{"zzz"}, []string{"zzz"}}

	b, err := json.Marshal(a)
	if err != nil {
		fmt.Print("Marshal error ", err)
	}
	fmt.Println("b = ", string(b))

	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutVarint(buf, int64(len(b)))
	fmt.Printf("%v %x\n", n, buf[:n])

	f, err := os.Create("x")
	if err != nil {
		log.Fatal(err)
	}

	_, err = f.Write(buf)
	if err != nil {
		log.Fatal(err)
	}
	_, err = f.Write(b)
	if err != nil {
		log.Fatal(err)
	}

	_, err = f.Write(buf)
	if err != nil {
		log.Fatal(err)
	}
	_, err = f.Write(b)
	if err != nil {
		log.Fatal(err)
	}
	f.Close()

	var a1 Attr
	f, err = os.Open("x")
	if err != nil {
		log.Fatal(err)
	}
	data := make([]byte, 8192)
	count, err := f.Read(data)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("read %d\n", count)

	r := bytes.NewReader(data[0:binary.MaxVarintLen64])
	len, err := binary.ReadVarint(r)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("len = %d %d\n", len, binary.MaxVarintLen64)

	data = data[binary.MaxVarintLen64:]
	err = json.Unmarshal(data[0:len], &a1)
	if err != nil {
		fmt.Print("Unmarshal error ", err)
	}

	data = data[len:]

	r = bytes.NewReader(data[0:binary.MaxVarintLen64])
	len, err = binary.ReadVarint(r)
	if err != nil {
		log.Fatal(err)
	}
	data = data[binary.MaxVarintLen64:]
	err = json.Unmarshal(data[:len], &a1)
	if err != nil {
		fmt.Print("Unmarshal error ", err)
	}
	fmt.Println("a1 = ", a1)

	f.Close()
}

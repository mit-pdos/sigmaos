package rand

import (
	crand "crypto/rand"
	"encoding/binary"
	"log"
	"math/big"

	"github.com/thanhpk/randstr"
)

const (
	NAME_LEN = 16 // Must be long enough to make collisions unlikely
)

func String(n int) string {
	return randstr.Hex(n)
}

func Name() string {
	return randstr.Hex(NAME_LEN)
}

func Uint64() uint64 {
	var b [8]byte
	_, err := crand.Read(b[:])
	if err != nil {
		log.Fatalf("crand.Read err %v\n", err)
	}
	return binary.LittleEndian.Uint64(b[:])
}

func Int64(max int64) uint64 {
	m := big.NewInt(max)
	rr, _ := crand.Int(crand.Reader, m)
	return rr.Uint64()
}

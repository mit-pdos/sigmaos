package main

import (
	"log"
	"math/rand"
	"strconv"
	"time"

	"ulambda/kv"
)

const (
	NKEYS   = 100
	NCLERK  = 10
	NTHREAD = 100
	T       = 10 * 1000
)

type Tstat struct {
	tot int64
	max int64
	n   int64
}

func zipf(r *rand.Rand) uint64 {
	z := rand.NewZipf(r, 2.0, 1.0, 99)
	return z.Uint64()
}

func uniform(r *rand.Rand) uint64 {
	return r.Uint64() % NKEYS
}

func key(k uint64) string {
	return "key" + strconv.FormatUint(k, 16)
}

func clerk(clk *kv.KvClerk, in chan bool, out chan Tstat, dist func(*rand.Rand) uint64) {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	st := Tstat{}
	for true {
		k := dist(rand)
		t0 := time.Now().UnixNano()
		v, err := clk.Get(key(k))
		t1 := time.Now().UnixNano()
		st.tot += t1 - t0
		if t1-t0 > st.max {
			st.max = t1 - t0
		}
		st.n += 1
		select {
		case <-in:
			out <- st
			return
		default:
			if err != nil {
				log.Fatalf("Get %v failed %v\n", key(k), err)
			}
			if key(k) != v {
				log.Fatalf("Get %v wrong val %v\n", key(k), v)
			}
		}
	}
}

func main() {
	clks := make([]*kv.KvClerk, NCLERK)
	in := make(chan bool)
	out := make(chan Tstat)

	for i := 0; i < NCLERK; i++ {
		clks[i] = kv.MakeClerk()
	}

	for i := uint64(0); i < NKEYS; i++ {
		err := clks[0].Put(key(i), key(i))
		if err != nil {
			log.Fatalf("Put %v failed %v\n", key(i), err)
		}
	}

	for i := 0; i < NCLERK; i++ {
		for t := 0; t < NTHREAD; t++ {
			go clerk(clks[i], in, out, uniform)
		}
	}

	time.Sleep(T * time.Millisecond)

	kvs := clks[0].KVs()
	if len(kvs) < 2 {
		log.Fatalf("KVs didn't expand %v\n", kvs)
	}

	stat := Tstat{}
	for i := 0; i < NCLERK*NTHREAD; i++ {
		in <- true
		st := <-out
		stat.n += st.n
		stat.tot += st.tot
		if st.max > stat.max {
			stat.max = st.max
		}
	}

	log.Printf("STATS n %v tput %v/s avg %v ns max %v ns\n", stat.n, stat.n/20,
		stat.tot/stat.n, stat.max)

	time.Sleep(5000 * time.Millisecond)

	kvs = clks[0].KVs()
	if len(kvs) != 1 {
		log.Fatalf("KVs didn't shrink %v\n", kvs)
	}

	//for i := 0; i < NCLERK; i++ {
	//	clks[i].Exit()
	//}

	log.Printf("PASS\n")
}

package netperf_test

import (
	"flag"
	"log"
	"net"
	"testing"
	"time"

	"github.com/montanaflynn/stats"
	"github.com/stretchr/testify/assert"
)

var srvaddr string
var ntrial int

func init() {
	flag.StringVar(&srvaddr, "srvaddr", ":8080", "Address of server.")
	flag.IntVar(&ntrial, "ntrial", 50, "Number of trials.")
}

func clntDialTCP(t *testing.T, addr string) {
	log.Printf("Client start dialing")
	lat := make([]float64, 0, ntrial)
	for i := 0; i < ntrial; i++ {
		start := time.Now()
		// Dial the listener
		conn, err := net.Dial("tcp", addr)
		assert.Nil(t, err, "Err Dial: %v", err)
		lat = append(lat, float64(time.Since(start).Microseconds()))
		err = conn.Close()
		assert.Nil(t, err, "Err Close: %v", err)
		time.Sleep(50 * time.Millisecond)
	}
	avgLat, err := stats.Mean(lat)
	assert.Nil(t, err, "Err Mean: %v", err)
	stdLat, err := stats.StandardDeviation(lat)
	assert.Nil(t, err, "Err Std: %v", err)
	log.Printf("Raw latency: %vus", lat)
	log.Printf("Mean latency: %vus", avgLat)
	log.Printf("Std latency: %vus", stdLat)
}

func srvDialTCP(t *testing.T, addr string) {
	l, err := net.Listen("tcp", addr)
	assert.Nil(t, err, "Err Listen: %v", err)
	log.Printf("Ready to accept connections")
	for i := 0; i < ntrial; i++ {
		conn, err := l.Accept()
		assert.Nil(t, err, "Err Accept: %v", err)
		err = conn.Close()
		assert.Nil(t, err, "Err Close: %v", err)
	}
	log.Printf("Done accepting connections")
}

func TestClntDialTCP(t *testing.T) {
	clntDialTCP(t, srvaddr)
}

func TestSrvDialTCP(t *testing.T) {
	srvDialTCP(t, srvaddr)
}

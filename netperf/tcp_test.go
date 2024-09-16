package netperf_test

import (
	"flag"
	"io"
	"log"
	"net"
	"testing"
	"time"

	"github.com/montanaflynn/stats"
	"github.com/stretchr/testify/assert"
)

const (
	MB = 1 << 20
)

var srvaddr string
var ntrial int
var bufsz int

func init() {
	flag.StringVar(&srvaddr, "srvaddr", ":8080", "Address of server.")
	flag.IntVar(&ntrial, "ntrial", 50, "Number of trials.")
	flag.IntVar(&bufsz, "bufsz", 1*MB, "Size of buffer in bytes.")
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

func clntThroughput(t *testing.T, conn net.Conn) {
	start := time.Now()
	b := make([]byte, bufsz)
	for i := 0; i < ntrial; i++ {
		n, err := conn.Write(b)
		assert.Nil(t, err, "Err Write: %v", err)
		assert.Equal(t, n, len(b), "Err short write: %v", n)
	}
	err := conn.Close()
	assert.Nil(t, err, "Err Close: %v", err)
	dur := time.Since(start)
	totBytes := int64(bufsz) * int64(ntrial)
	log.Printf("Total bytes: %v", totBytes)
	log.Printf("Elapsed time: %v", dur)
	log.Printf("Throughput: %vMB/s", float64(totBytes/MB)/dur.Seconds())
}

func srvThroughput(t *testing.T, conn net.Conn) {
	start := time.Now()
	b := make([]byte, bufsz)
	for i := 0; i < ntrial; i++ {
		n, err := io.ReadFull(conn, b)
		assert.Nil(t, err, "Err Read: %v", err)
		assert.Equal(t, n, len(b), "Err short read: %v", n)
	}
	err := conn.Close()
	assert.Nil(t, err, "Err Close: %v", err)
	dur := time.Since(start)
	totBytes := int64(bufsz) * int64(ntrial)
	log.Printf("Total bytes: %v", totBytes)
	log.Printf("Elapsed time: %v", dur)
	log.Printf("Throughput: %vMB/s", float64(totBytes/MB)/dur.Seconds())
}

func TestClntThroughputTCP(t *testing.T) {
	conn, err := net.Dial("tcp", srvaddr)
	if !assert.Nil(t, err, "Err Dial: %v", err) {
		return
	}
	clntThroughput(t, conn)
}

func TestSrvThroughputTCP(t *testing.T) {
	l, err := net.Listen("tcp", srvaddr)
	if !assert.Nil(t, err, "Err Listen: %v", err) {
		return
	}
	log.Printf("Ready to accept connections")
	conn, err := l.Accept()
	assert.Nil(t, err, "Err Accept: %v", err)
	srvThroughput(t, conn)
}

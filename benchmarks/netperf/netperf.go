package netperf

import (
	"fmt"
	"time"

	"github.com/montanaflynn/stats"

	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	sp "sigmaos/sigmap"
)

func SrvDialDialProxy(started chan bool, ntrial int, npc *dialproxyclnt.DialProxyClnt, addr *sp.Taddr, epType sp.TTendpoint) error {
	_, l, err := npc.Listen(epType, addr)
	if err != nil {
		return err
	}
	db.DPrintf(db.TEST, "Ready to accept connections")
	started <- true
	for i := 0; i < ntrial; i++ {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		if err := conn.Close(); err != nil {
			return err
		}
	}
	db.DPrintf(db.TEST, "Done accepting connections")
	return nil
}

func ClntDialDialProxy(ntrial int, npc *dialproxyclnt.DialProxyClnt, ep *sp.Tendpoint) (string, error) {
	db.DPrintf(db.TEST, "Client start dialing")
	lat := make([]float64, 0, ntrial)
	for i := 0; i < ntrial; i++ {
		start := time.Now()
		// Dial the listener
		conn, err := npc.Dial(ep)
		if err != nil {
			return "", err
		}
		lat = append(lat, float64(time.Since(start).Microseconds()))
		if err := conn.Close(); err != nil {
			return "", err
		}
		time.Sleep(50 * time.Millisecond)
	}
	avgLat, err := stats.Mean(lat)
	if err != nil {
		return "", err
	}
	stdLat, err := stats.StandardDeviation(lat)
	if err != nil {
		return "", err
	}
	maxLat, err := stats.Max(lat)
	if err != nil {
		return "", err
	}
	outStr := ""
	db.DPrintf(db.BENCH, "Raw latency: %vus", lat)
	outStr += fmt.Sprintf("Raw latency: %vus\n", lat)
	db.DPrintf(db.BENCH, "Max latency: %vus", maxLat)
	outStr += fmt.Sprintf("Max latency: %vus\n", maxLat)
	db.DPrintf(db.BENCH, "Mean latency: %vus", avgLat)
	outStr += fmt.Sprintf("Mean latency: %vus\n", avgLat)
	db.DPrintf(db.BENCH, "Std latency: %vus", stdLat)
	outStr += fmt.Sprintf("Std latency: %vus", stdLat)
	return outStr, nil
}

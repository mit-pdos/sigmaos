package sessclnt

import (
	"sync"
	"time"

	db "ulambda/debug"
	np "ulambda/ninep"
)

type Heartbeater struct {
	sync.Mutex
	sc            *SessClnt
	lastHeartbeat time.Time
	done          bool
}

func makeHeartbeater(sc *SessClnt) *Heartbeater {
	h := &Heartbeater{}
	h.sc = sc
	go h.run()
	return h
}

// A heartbeat was ack'd by the server
func (h *Heartbeater) HeartbeatAckd() {
	h.Lock()
	defer h.Unlock()
	h.lastHeartbeat = time.Now()
}

// Stop sending heartbeats
func (h *Heartbeater) Stop() {
	h.Lock()
	defer h.Unlock()
	h.done = true
}

func (h *Heartbeater) isDone() bool {
	h.Lock()
	defer h.Unlock()
	return h.done
}

func (h *Heartbeater) needsHeartbeat() bool {
	h.Lock()
	defer h.Unlock()
	return !h.done && time.Now().Sub(h.lastHeartbeat) >= np.Conf.Session.HEARTBEAT_INTERVAL
}

func (h *Heartbeater) run() {
	for !h.isDone() {
		// Sleep a bit.
		time.Sleep(np.Conf.Session.HEARTBEAT_INTERVAL)
		if h.needsHeartbeat() {
			// XXX How soon should I retry if this fails?
			db.DPrintf("SESSCLNT", "%v Sending heartbeat to %v", h.sc.sid, h.sc.addrs)
			_, err := h.sc.RPC(np.Theartbeat{[]np.Tsession{h.sc.sid}}, np.NoFence)
			if err != nil {
				db.DPrintf("SESSCLNT_ERR", "%v heartbeat %v err %v", h.sc.sid, h.sc.addrs, err)
			}
		}
	}
}

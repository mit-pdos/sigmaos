package spchannel

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// A channel's init is slow (it opens an RPC fd on the server), and callers race
// through it: a mapper's chunk readers all issue their first get concurrently.
// init must not publish "done" until fd is set, or a caller taking the lock-free
// fast path sends on fd 0 and the server rejects it as an unknown fid.
//
// This exercises the same publish-once discipline as SPChannel.init, with the
// I/O replaced by a sleep, so it can run without a kernel. Run with -race to
// catch the unsynchronized flag read as well.
type fakeChannel struct {
	mu       sync.Mutex
	fd       int
	initDone atomic.Bool
	initErr  error
	ninit    atomic.Int32
	failInit bool
}

func (ch *fakeChannel) init() (err error) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if ch.initDone.Load() {
		return ch.initErr
	}
	defer func() {
		ch.initErr = err
		ch.initDone.Store(true)
	}()
	ch.ninit.Add(1)
	// Stand-in for NewSessDevClnt + Open: the window in which a racing caller
	// used to observe initDone with fd still unset.
	time.Sleep(2 * time.Millisecond)
	if ch.failInit {
		return errors.New("init failed")
	}
	ch.fd = 7 // a valid fd; 0 is the "never opened" value
	return nil
}

func (ch *fakeChannel) checkInit() error {
	if ch.initDone.Load() {
		return ch.initErr
	}
	return ch.init()
}

// send is SendReceive's shape: check init, then use fd.
func (ch *fakeChannel) send() (int, error) {
	if err := ch.checkInit(); err != nil {
		return 0, err
	}
	return ch.fd, nil
}

func TestChannelInitRace(t *testing.T) {
	for range 50 {
		ch := &fakeChannel{}
		var wg sync.WaitGroup
		bad := atomic.Int32{}
		for range 5 { // CONCURRENCY chunk readers
			wg.Add(1)
			go func() {
				defer wg.Done()
				fd, err := ch.send()
				if err != nil {
					t.Errorf("send: %v", err)
				}
				if fd == 0 {
					// The unknown-fid bug: init reported done before fd was set.
					bad.Add(1)
				}
			}()
		}
		wg.Wait()
		if n := bad.Load(); n != 0 {
			t.Fatalf("%d callers sent on an unset fd", n)
		}
		if n := ch.ninit.Load(); n != 1 {
			t.Fatalf("init ran %d times, want exactly 1", n)
		}
	}
}

// An init failure must reach every caller, not be swallowed into a send on an
// unset fd, and must not be retried per call.
func TestChannelInitFailure(t *testing.T) {
	ch := &fakeChannel{failInit: true}
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := ch.send(); err == nil {
				t.Errorf("send succeeded despite init failure")
			}
		}()
	}
	wg.Wait()
	if _, err := ch.send(); err == nil {
		t.Errorf("later send succeeded despite init failure")
	}
	if n := ch.ninit.Load(); n != 1 {
		t.Errorf("init ran %d times, want exactly 1 (failure is remembered)", n)
	}
}

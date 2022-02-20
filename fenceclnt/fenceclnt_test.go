package fenceclnt_test

import (
	"log"

	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"ulambda/fenceclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/test"
)

//
// Fence tests with only named. Other basic tests in kernel and
// procclnt.
//

const (
	FENCE_DIR = "name/fence"
	FENCENAME = "name/test-fence"
)

func TestFence1(t *testing.T) {
	ts := test.MakeTstate(t)

	N := 20
	sum := 0
	current := 0
	done := make(chan int)

	fence := fenceclnt.MakeFenceClnt(ts.FsLib, FENCENAME, 0, []string{np.NAMED})

	for i := 0; i < N; i++ {
		go func(i int) {
			me := false
			for !me {
				err := fence.AcquireFenceW([]byte{})
				assert.Nil(ts.T, err, "AcquireFenceW")
				if current == i {
					me = true
				}
				err = fence.ReleaseFence()
				assert.Nil(ts.T, err, "ReleaseFence")
				if me {
					current += 1
					done <- i
				}
			}
		}(i)
		sum += i
	}

	for i := 0; i < N; i++ {
		next := <-done
		assert.Equal(ts.T, i, next, "Next (%v) not equal to expected (%v)", next, i)
	}

	ts.Shutdown()
}

func TestFence2(t *testing.T) {
	ts := test.MakeTstate(t)

	N := 20

	fence1 := fenceclnt.MakeFenceClnt(ts.FsLib, FENCENAME+"-1234", 0, []string{np.NAMED})
	fence2 := fenceclnt.MakeFenceClnt(ts.FsLib, FENCENAME+"-1234", 0, []string{np.NAMED})

	for i := 0; i < N; i++ {
		err := fence1.AcquireFenceW([]byte{})
		assert.Nil(ts.T, err, "AcquireFenceW")
		err = fence1.ReleaseFence()
		assert.Nil(ts.T, err, "ReleaseFence")
		err = fence2.AcquireFenceW([]byte{})
		assert.Nil(ts.T, err, "AcquireFenceW")
		err = fence2.ReleaseFence()
		assert.Nil(ts.T, err, "ReleaseFence")
	}

	ts.Shutdown()
}

// n threads help to increase cnt to N
func TestFence3(t *testing.T) {
	ts := test.MakeTstate(t)

	N := 3000
	n_threads := 20
	cnt := 0

	fence := fenceclnt.MakeFenceClnt(ts.FsLib, FENCENAME+"-1234", 0, []string{np.NAMED})

	var done sync.WaitGroup
	done.Add(n_threads)

	for i := 0; i < n_threads; i++ {
		go func(done *sync.WaitGroup, fence *fenceclnt.FenceClnt, N *int, cnt *int) {
			defer done.Done()
			for {
				err := fence.AcquireFenceW([]byte{})
				assert.Nil(ts.T, err, "AcquireFence")
				if *cnt < *N {
					*cnt += 1
				} else {
					err = fence.ReleaseFence()
					assert.Nil(ts.T, err, "ReleaseFence")
					break
				}
				err = fence.ReleaseFence()
				assert.Nil(ts.T, err, "ReleaseFence")
			}
		}(&done, fence, &N, &cnt)
	}

	done.Wait()
	assert.Equal(ts.T, N, cnt, "Count doesn't match up")

	ts.Shutdown()
}

// Test if an exit of another session doesn't remove ephemeral files
// of another session.
func TestFence4(t *testing.T) {
	ts := test.MakeTstate(t)

	fsl1 := fslib.MakeFsLibAddr("fslib-1", fslib.Named())
	fsl2 := fslib.MakeFsLibAddr("fslib-2", fslib.Named())

	fence1 := fenceclnt.MakeFenceClnt(fsl1, FENCENAME, 0, []string{np.NAMED})

	// Establish a connection
	err := fsl2.Mkdir(FENCE_DIR, 07)
	assert.Nil(ts.T, err, "ReadDir")

	err = fence1.AcquireFenceW([]byte{})
	assert.Nil(ts.T, err, "AcquireFenceW")

	fsl2.Exit()

	time.Sleep(2 * time.Second)

	err = fence1.ReleaseFence()
	assert.Nil(ts.T, err, "ReleaseFence")
	ts.Shutdown()
}

// Inline Set() so that we can delay the Write() to emulate a delay on
// the server between open and write.
func writeFile(fl *fslib.FsLib, fn string, d []byte) error {
	fd, err := fl.Open(fn, np.OWRITE)
	if err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	_, err = fl.Write(fd, d)
	if err != nil {
		return err
	}
	err = fl.Close(fd)
	if err != nil {
		return err
	}
	return nil
}

// Caller must have acquired fence and keeps writing until open fails
// or write fails because stale fence
func write(fsl *fslib.FsLib, ch chan int, fn string) {
	const N = 1000
	for i := 1; i < N; {
		d := []byte(strconv.Itoa(i))
		err := writeFile(fsl, fn, d)
		if err == nil {
			i++
		} else {
			log.Printf("write %v err %v\n", i, err)
			ch <- i - 1
			return
		}
	}
	ch <- N - 1
}

func writer(t *testing.T, ch chan int, N int, fn string) {
	fsl := fslib.MakeFsLibAddr("fsl1", fslib.Named())
	f := fenceclnt.MakeFenceClnt(fsl, "name/config", 0, []string{np.NAMED})
	cont := true
	for cont {
		// Wait for fence to exist, indicating a new
		// iteration.
		b, err := f.AcquireFenceR()
		assert.Equal(t, nil, err)

		n, err := strconv.Atoi(string(b))
		write(fsl, ch, fn)

		if n == N-1 {
			cont = false
		}

		err = f.ReleaseFence()
		assert.Equal(t, nil, err)
	}
}

// Test fences with two fsclnts. One makes a write fence for an epoch
// file. The other fsclnt opens another file and writes to it under a
// read fence for the epoch file.  If the first fsclnt changes the
// fence (incrementing the epoch) between the second fsclnt opening
// and writing the other file, the write should fail with stale error,
// because the read fence isn't valid anymore.
func TestSetRenameGet(t *testing.T) {
	const N = 100

	ts := test.MakeTstate(t)

	fn := "name/f"
	fn1 := "name/f1"
	d := []byte(strconv.Itoa(0))
	_, err := ts.PutFile(fn, 0777, np.OWRITE, d)
	assert.Equal(t, nil, err)

	ch := make(chan int)
	f := fenceclnt.MakeFenceClnt(ts.FsLib, "name/config", 0, []string{np.NAMED})

	// Make new fence with iteration number
	err = f.AcquireFenceW([]byte(strconv.Itoa(0)))
	assert.Equal(t, nil, err)

	go writer(t, ch, N, fn)

	for i := 0; i < N; i++ {

		// Let the writer write for some time
		time.Sleep(100 * time.Millisecond)

		// Now rename so writer cannot open the file
		err = ts.Rename(fn, fn1)
		assert.Equal(t, nil, err)

		// Update the fence to next iteration so that any
		// writer operation under read fence will fail, if
		// writer opened before rename/SetFenceFile.
		err = f.SetFenceFile([]byte(strconv.Itoa(i + 1)))
		assert.Equal(t, nil, err)

		// check that writer didn't get its write in after
		// setfencefile

		d1, err := ts.GetFile(fn1)
		n, err := strconv.Atoi(string(d1))
		assert.Equal(t, nil, err)

		m := <-ch

		if n != m {
			assert.Equal(t, n, m)
			break
		}

		// Rename back and do another iteration of testing
		err = ts.Rename(fn1, fn)
		assert.Equal(t, nil, err)
	}

	err = f.ReleaseFence()
	assert.Equal(t, nil, err)

	ts.Shutdown()
}

func primary(t *testing.T, ch chan bool, i int) {
	n := strconv.Itoa(i)
	fsl := fslib.MakeFsLibAddr("fsl"+n, fslib.Named())
	f := fenceclnt.MakeFenceClnt(fsl, "name/config", 0, []string{np.NAMED})

	err := f.AcquireFenceW([]byte{})
	assert.Nil(t, err, "AcquireFenceW")

	err = f.SetFenceFile([]byte(n))
	assert.Nil(t, err, "SetFenceFile")

	fn := "name/f"
	d := []byte(n)
	_, err = fsl.PutFile(fn, 0777, np.OWRITE, d)

	err = f.MakeFenceFileFrom(fn)
	assert.Nil(t, err, "MakeFenceFileFrom")

	fsl.Exit() // crash

	ch <- true
}

func TestCrashPrimary(t *testing.T) {
	ts := test.MakeTstate(t)
	N := 3

	ch := make(chan bool)

	for i := 0; i < N; i++ {
		go primary(ts.T, ch, i)
	}

	for i := 0; i < N; i++ {
		<-ch
	}

	ts.Shutdown()
}

func TestRemoveFence(t *testing.T) {
	ts := test.MakeTstate(t)

	fence := fenceclnt.MakeFenceClnt(ts.FsLib, FENCENAME, 0, []string{np.NAMED})

	err := fence.AcquireFenceW([]byte{})
	assert.Nil(ts.T, err, "AcquireFenceW")

	f, err := fence.Fence()
	assert.Nil(ts.T, err, "Fence")

	err = fence.ReleaseFence()
	assert.Nil(ts.T, err, "ReleaseFence")

	err = fence.RemoveFence()
	assert.Nil(ts.T, err, "RmFence")

	fence1 := fenceclnt.MakeFenceClnt(ts.FsLib, FENCENAME, 0, []string{np.NAMED})

	err = fence1.AcquireFenceW([]byte{})
	assert.Nil(ts.T, err, "AcquireFenceW")

	g, err := fence1.Fence()
	assert.Nil(ts.T, err, "Fence")

	assert.Equal(ts.T, f.Seqno, g.Seqno, "AcquireFenceW")

	ts.Shutdown()
}

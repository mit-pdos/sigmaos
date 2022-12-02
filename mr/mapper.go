package mr

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	// "runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	"sigmaos/crash"
	db "sigmaos/debug"
	"sigmaos/fslib"
	np "sigmaos/sigmap"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/rand"
	"sigmaos/test"
)

type Mapper struct {
	*fslib.FsLib
	*procclnt.ProcClnt
	mapf        MapT
	job         string
	nreducetask int
	linesz      int
	input       string
	bin         string
	wrts        []*fslib.Wrt
	rand        string
	perf        *perf.Perf
}

func MkMapper(mapf MapT, job string, p *perf.Perf, nr, lsz int, input string) *Mapper {
	m := &Mapper{}
	m.mapf = mapf
	m.job = job
	m.nreducetask = nr
	m.linesz = lsz
	m.rand = rand.String(16)
	m.input = input
	m.bin = path.Base(m.input)
	m.wrts = make([]*fslib.Wrt, m.nreducetask)
	m.FsLib = fslib.MakeFsLib("mapper-" + proc.GetPid().String() + " " + m.input)
	m.perf = p
	return m
}

func makeMapper(mapf MapT, args []string, p *perf.Perf) (*Mapper, error) {
	if len(args) != 4 {
		return nil, fmt.Errorf("MakeMapper: too few arguments %v", args)
	}
	nr, err := strconv.Atoi(args[1])
	if err != nil {
		return nil, fmt.Errorf("MakeMapper: nreducetask %v isn't int", args[1])
	}
	lsz, err := strconv.Atoi(args[3])
	if err != nil {
		return nil, fmt.Errorf("MakeMapper: linesz %v isn't int", args[1])
	}
	m := MkMapper(mapf, args[0], p, nr, lsz, args[2])
	m.ProcClnt = procclnt.MakeProcClnt(m.FsLib)
	if err := m.Started(); err != nil {
		return nil, fmt.Errorf("MakeMapper couldn't start %v", args)
	}
	crash.Crasher(m.FsLib)
	return m, nil
}

func (m *Mapper) CloseWrt() (np.Tlength, error) {
	nout, err := m.closewrts()
	if err != nil {
		return 0, err
	}
	return nout, nil
}

func (m *Mapper) InitWrt(r int, name string) error {
	if wrt, err := m.CreateAsyncWriter(name, 0777, np.OWRITE); err != nil {
		return err
	} else {
		m.wrts[r] = wrt
	}
	return nil
}

func (m *Mapper) initMapper() error {
	// Make a directory for holding the output files of a map task.  Ignore
	// error in case it already exits.  XXX who cleans up?
	m.MkDir(MLOCALDIR, 0777)
	m.MkDir(LocalOut(m.job), 0777)
	m.MkDir(Moutdir(m.job, m.bin), 0777)

	// Create the output files
	for r := 0; r < m.nreducetask; r++ {
		// create temp output shard for reducer r
		oname := mshardfile(m.job, m.bin, r) + m.rand
		if err := m.InitWrt(r, oname); err != nil {
			m.closewrts()
			return err
		}
	}
	return nil
}

func (m *Mapper) closewrts() (np.Tlength, error) {
	n := np.Tlength(0)
	for r := 0; r < m.nreducetask; r++ {
		if m.wrts[r] != nil {
			if err := m.wrts[r].Close(); err != nil {
				return 0, err
			} else {
				n += m.wrts[r].Nbytes()
			}
		}
	}
	return n, nil
}

// Inform reducer where to find map output
func (m *Mapper) informReducer() error {
	st, err := m.Stat(MLOCALSRV)
	if err != nil {
		return fmt.Errorf("%v: stat %v err %v\n", proc.GetName(), MLOCALSRV, err)
	}
	for r := 0; r < m.nreducetask; r++ {
		fn := mshardfile(m.job, m.bin, r)
		err = m.Rename(fn+m.rand, fn)
		if err != nil {
			return fmt.Errorf("%v: rename %v -> %v err %v\n", proc.GetName(), fn+m.rand, fn, err)
		}

		name := symname(m.job, strconv.Itoa(r), m.bin)

		// Remove name in case an earlier mapper created the
		// symlink.  A reducer may have opened and is reading
		// the old target, open the new input file and read
		// the new target, or fail because there is no
		// symlink. Failing is fine because the coodinator
		// will start a new reducer once this map completes.
		// We could use rename to atomically remove and create
		// the symlink if we want to avoid the failing case.
		m.Remove(name)

		target := shardtarget(m.job, st.Name, m.bin, r)

		db.DPrintf("MR", "name %s target %s\n", name, target)

		err = m.Symlink([]byte(target), name, 0777)
		if err != nil {
			db.DFatalf("%v: FATAL symlink %v err %v\n", proc.GetName(), name, err)
		}
	}
	return nil
}

func (m *Mapper) emit(kv *KeyValue) error {
	r := Khash(kv.Key) % m.nreducetask
	return encodeKV(m.wrts[r], kv.Key, kv.Value, r)
	//	b, err := json.Marshal(kv)
	//	if err != nil {
	//		return fmt.Errorf("%v: mapper %v err %v", proc.GetName(), r, err)
	//	}

	//	if n, err := m.wrts[r].bwrt.Write(b); err != nil || n != len(b) {
	//		return fmt.Errorf("%v: mapper %v write err %v", proc.GetName(), r, err)
	//	}

	//	if err := fslib.WriteJsonRecord(m.wrts[r].bwrt, kv); err != nil {
	//		return fmt.Errorf("%v: mapper %v err %v", proc.GetName(), r, err)
	//	}
}

func (m *Mapper) DoSplit(s *Split) (np.Tlength, error) {
	off := s.Offset
	if off != 0 {
		// -1 to pick up last byte from prev split so that if s.Offset
		// != 0 below works out correctly. if the last byte of
		// previous split is a newline, this mapper should process the
		// first line of the split.  if not, this mapper should ignore
		// the first line of the split because it has been processed
		// as part of the previous split.
		off--
	}
	rdr, err := m.OpenAsyncReader(s.File, off)
	if err != nil {
		db.DFatalf("%v: read %v err %v", proc.GetName(), s.File, err)
	}
	defer rdr.Close()
	scanner := bufio.NewScanner(rdr)
	buf := make([]byte, 0, m.linesz)
	scanner.Buffer(buf, cap(buf))

	// advance scanner to new line after start, if start != 0
	n := 0
	if s.Offset != 0 {
		scanner.Scan()
		l := scanner.Text()
		n += len(l) // +1 for newline, but -1 for extra byte we read
	}
	for scanner.Scan() {
		l := scanner.Text()
		n += len(l) + 1 // 1 for newline
		if len(l) > 0 {
			if err := m.mapf(m.input, strings.NewReader(l), m.emit); err != nil {
				return 0, err
			}
		}
		m.perf.TptTick(float64(len(l)))
		if np.Tlength(n) >= s.Length {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return np.Tlength(n), err
	}
	return np.Tlength(n), nil
}

func (m *Mapper) doMap() (np.Tlength, np.Tlength, error) {
	db.DPrintf(db.ALWAYS, "doMap %v", m.input)
	rdr, err := m.OpenReader(m.input)
	if err != nil {
		return 0, 0, err
	}
	dec := json.NewDecoder(rdr)
	ni := np.Tlength(0)
	for {
		var s Split
		if err := dec.Decode(&s); err == io.EOF {
			break
		} else if err != nil {
			c, _ := m.GetFile(m.input)
			db.DPrintf("MR", "Mapper %s: decode %v err %v\n", m.bin, string(c), err)
			return 0, 0, err
		}
		db.DPrintf("MR", "Mapper %s: process split %v\n", m.bin, s)
		n, err := m.DoSplit(&s)
		if err != nil {
			db.DPrintf("MR", "doSplit %v err %v\n", s, err)
			return 0, 0, err
		}
		if n < s.Length {
			db.DFatalf("Split: short split o %d l %d %d\n", s.Offset, s.Length, n)
		}
		ni += n
	}
	nout, err := m.CloseWrt()
	if err != nil {
		return 0, 0, err
	}
	if err := m.informReducer(); err != nil {
		return 0, 0, err
	}
	return ni, nout, nil
}

func RunMapper(mapf MapT, args []string) {
	p := perf.MakePerf("MRMAPPER")
	defer p.Done()

	// debug.SetMemoryLimit(1769 * 1024 * 1024)

	m, err := makeMapper(mapf, args, p)
	if err != nil {
		db.DFatalf("%v: error %v", os.Args[0], err)
	}
	if err = m.initMapper(); err != nil {
		m.Exited(proc.MakeStatusErr(err.Error(), nil))
		return
	}
	start := time.Now()
	nin, nout, err := m.doMap()
	db.DPrintf("MRTPT", "%s: in %s out %s %vms (%s)\n", "map", humanize.Bytes(uint64(nin)), humanize.Bytes(uint64(nout)), time.Since(start).Milliseconds(), test.TputStr(nin+nout, time.Since(start).Milliseconds()))
	if err == nil {
		m.Exited(proc.MakeStatusInfo(proc.StatusOK, m.input,
			Result{true, m.input, nin, nout, time.Since(start).Milliseconds()}))
	} else {
		m.Exited(proc.MakeStatusErr(err.Error(), nil))
	}
}

package srv

//
// This file is based on criu/uffd.c
//

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/checkpoint-restore/go-criu/v7/crit"
	"github.com/checkpoint-restore/go-criu/v7/crit/cli"
	"github.com/checkpoint-restore/go-criu/v7/crit/images/inventory"
	"github.com/checkpoint-restore/go-criu/v7/crit/images/mm"
	"github.com/checkpoint-restore/go-criu/v7/crit/images/pagemap"
	"github.com/checkpoint-restore/go-criu/v7/crit/images/pstree"

	db "sigmaos/debug"
)

const (
	PREFETCH = 16 // number of pages to prefetch
)

var saved_addresses = [][2]uint64{
	{0x1be7000, 9},
	{0x762047a95000, 9},
	{0xc000075000, 5},
	{0x762047a2c000, 1},
	{0x1c48000, 3},
	{0xc00007e000, 1},
	{0xc000006000, 9},
	{0xc00009a000, 3},
	{0xc000098000, 1},
	{0xc00005c000, 11},
	{0xc000014000, 11},
	{0x1c00000, 8},
	{0x762047be7000, 2},
	{0x762000fbe000, 10},
	{0xc000208000, 13},
	{0x762047a72000, 1},
	{0xc0033a5000, 5},
	{0x7ffe08af1000, 3},
	{0x76200107f000, 9},
	{0xc0000b3000, 17},
	{0xc00013a000, 12},
	{0x762047ce7000, 3},
	{0xc00056f000, 17},
	{0xc00025e000, 3},
	{0xc000040000, 7},
	{0x1c4f000, 3},
	{0xc000065000, 9},
	{0xc0001f4000, 6},
	{0x762000d25000, 17},
	{0x762000c82000, 17},
	{0x762000d32000, 2},
	{0xc003dfe000, 13},
	{0xc00333a000, 17},
	{0xc0033e0000, 13},
	{0xc000373000, 17},
	{0xc0003c9000, 11},
	{0xc003db8000, 13},
	{0xc003384000, 13},
	{0x76200100b000, 12},
	{0x762047bf1000, 9},
	{0xc0003bc000, 9},
	{0xc000172000, 9},
	{0xc000242000, 7},
	{0xc000174000, 9},
	{0x762000d2e000, 3},
	{0xc0001be000, 5},
	{0xc000210000, 5},
	{0xc00035a000, 5},
	{0xc00010a000, 4},
	{0xc0001b4000, 9},
	{0xc00018c000, 9},
	{0xc0003b1000, 16},
	{0xc000055000, 9},
	{0xc003351000, 3},
	{0x762000c8e000, 12},
	{0xc0003d7000, 5},
	{0xc003509000, 10},
	{0xc0032fe000, 15},
	{0xc00004a000, 5},
	{0xc0030c9000, 15},
	{0xc000036000, 16},
	{0xc00008f000, 2},
	{0xc0000a3000, 1},
	{0x762000d38000, 2},
	{0xc0001aa000, 10},
	{0xc0003a4000, 11},
	{0x762000e0f000, 14},
	{0xc002d31000, 17},
	{0x762000dce000, 17},
	{0xc003d87000, 17},
	{0x762000e50000, 17},
	{0xc00329b000, 17},
	{0xc00022a000, 7},
	{0xc00325d000, 17},
	{0x7620010a0000, 17},
	{0xc002f65000, 17},
	{0x762000cbc000, 11},
	{0xc000256000, 13},
	{0x1bf0000, 2},
	{0x7620010c3000, 12},
	{0xc0001b9000, 5},
	{0x762002218000, 1},
	{0xc00012d000, 10},
	{0x762047ab6000, 9},
	{0x762000cf2000, 14},
	{0x762047ac6000, 8},
	{0x762000ffa000, 9},
	{0x762000c39000, 1},
	{0x762000e07000, 7},
	{0x762000fc7000, 2},
	{0x762000fba000, 1},
	{0xc00021e000, 5},
	{0x555578595000, 1},
	{0x7ffe08af3000, 1},
	{0x762047c42000, 1},
	{0xc0001d5000, 2},
	{0x762000c5b000, 6},
	{0x762000cac000, 13},
	{0xc00028f000, 17},
	{0xc00340a000, 13},
	{0xc000514000, 10},
	{0xc0015d1000, 17},
	{0xc003df0000, 13},
	{0xc000351000, 16},
	{0xc00330a000, 11},
	{0xc0001d0000, 7},
	{0x762000d04000, 17},
	{0xc003416000, 11},
	{0xc003c56000, 9},
	{0xc003d00000, 7},
	{0xc000122000, 3},
	{0xc000264000, 9},
	{0xc000363000, 11},
	{0x762000d79000, 17},
	{0x762001031000, 17},
	{0xc00018e000, 9},
	{0x762000cb3000, 4},
	{0x762000e2e000, 17},
	{0xc0001d8000, 9},
	{0xc000323000, 10},
	{0xc000182000, 4},
	{0xc003de4000, 8},
	{0xc00029b000, 4},
	{0x762001088000, 9},
	{0x762033380000, 1},
	{0x762047bd7000, 1},
	{0x762047b47000, 1},
	{0x762047606000, 1},
	{0x762045230000, 1},
	{0x1bf8000, 1},
	{0x762001100000, 1},
	{0x762013380000, 1},
	{0x762000c49000, 1},
	{0xc00334e000, 1},
	{0xc00334c000, 9},
	{0xc0001e3000, 8},
	{0xc003dde000, 9},
	{0xc00027c000, 9},
	{0xc000309000, 12},
	{0xc00342a000, 9},
	{0x762001004000, 2},
	{0xc0033ee000, 8},
	{0xc00011c000, 6},
	{0xc000218000, 1},
	{0xc000130000, 3},
	{0xc0004b5000, 17},
	{0xc0033c6000, 17},
	{0x762000e3a000, 12},
	{0x762000def000, 17},
	{0xc001668000, 17},
	{0xc00025c000, 1},
	{0xc0003f0000, 2},
	{0xc00338c000, 11},
	{0xc0033b6000, 16},
	{0xc003322000, 17},
	{0xc00014c000, 17},
	{0xc00250c000, 17},
	{0x762000e19000, 10},
	{0xc00051c000, 9},
	{0xc0033d6000, 12},
	{0xc00037c000, 9},
	{0xc000166000, 12},
	{0xc0033a2000, 9},
	{0x762000cd0000, 17},
	{0x762047a4f000, 1},
	{0x7620010aa000, 10},
	{0xc003482000, 15},
	{0xc003492000, 13},
	{0xc00349f000, 12},
	{0xc0034aa000, 3},
	{0xc0034c8000, 9},
	{0xc0001a0000, 10},
	{0xc0034b3000, 10},
	{0xc0034d3000, 12},
	{0xc000283000, 9},
	{0xc000118000, 1},
	{0xc0034e6000, 1},
	{0x762000e46000, 5},
	{0xc00017e000, 2},
	{0xc0034ef000, 10},
	{0xc0002a2000, 9},
	{0xc003500000, 9},
	{0xc00053d000, 17},
	{0xc00351c000, 9},
	{0x762001014000, 9},
	{0xc000116000, 3},
	{0xc003534000, 10},
	{0x7620010ce000, 11},
	{0xc00336c000, 9},
	{0xc003828000, 1},
	{0x762000dba000, 9},
	{0xc0003f8000, 9},
	{0xc000313000, 9},
	{0xc000231000, 2},
	{0xc003358000, 11},
	{0xc003361000, 3},
	{0x762000cc6000, 3},
}

func nPages(start, end uint64, pagesz int) int {
	len := end - start
	return int((len + uint64(pagesz) - 1) / uint64(pagesz))
}

func ReadImg(imgdir, id string, magic string) (*crit.CriuImage, error) {
	pn := filepath.Join(imgdir, magic)
	if id == "" {
		pn = pn + ".img"
	} else {
		pn = pn + "-" + id + ".img"
	}
	f, err := os.Open(pn)
	if err != nil {
		return nil, err
	}
	c := crit.New(f, nil, "", false, false)
	entryType, err := cli.GetEntryTypeFromImg(f)
	if err != nil {
		return nil, fmt.Errorf("Unknown Entry type %q: %w", pn, err)
	}
	img, err := c.Decode(entryType)
	if err != nil {
		return nil, err
	}
	return img, nil
}

type Tinventory struct {
	*inventory.InventoryEntry
}

func NewTinventory(imgdir string) (*Tinventory, error) {
	img, err := ReadImg(imgdir, "", "inventory")
	if err != nil {
		return nil, err
	}
	e := img.Entries[0].Message
	i := &Tinventory{e.(*inventory.InventoryEntry)}
	return i, nil
}

type Tpstree struct {
	PstreeEntries []*crit.CriuEntry
}

func NewTpstree(imgdir string) (*Tpstree, error) {
	img, err := ReadImg(imgdir, "", "pstree")
	if err != nil {
		return nil, err
	}
	ps := &Tpstree{img.Entries}
	return ps, nil
}

func (ps *Tpstree) RootPid() int {
	e := ps.PstreeEntries[0]
	p := e.Message.(*pstree.PstreeEntry)
	return int(p.GetPid())
}

type TpagemapImg struct {
	pagesz         int
	PageMapHead    *crit.CriuEntry
	PagemapEntries []*crit.CriuEntry
	pagePrefix     []int
	nopages        int
}

func newTpagemapImg(imgdir, id string) (*TpagemapImg, error) {
	img, err := ReadImg(imgdir, id, "pagemap")
	if err != nil {
		return nil, err
	}
	prefix := make([]int, len(img.Entries)-1)
	sm := 0
	for i, pme := range img.Entries[1:] {
		prefix[i] = sm
		sm += int(pme.Message.(*pagemap.PagemapEntry).GetNrPages())
	}
	db.DPrintf(db.CKPT, "Total Pages: %v", sm)
	return &TpagemapImg{
			pagesz:         os.Getpagesize(),
			PageMapHead:    img.Entries[0],
			PagemapEntries: img.Entries[1:],
			nopages:        sm,
			pagePrefix:     prefix},

		nil
}

func (pmi *TpagemapImg) findBinSearch(addr uint64) int {
	low := 0
	high := len(pmi.PagemapEntries) - 1
	for low <= high {

		mid := (low + high) >> 1
		pm := pmi.PagemapEntries[mid].Message.(*pagemap.PagemapEntry)
		n := pm.GetNrPages()
		start := pm.GetVaddr()
		if start > addr {
			high = mid - 1
		} else if start+uint64(n*uint32(pmi.pagesz)) <= addr {
			low = mid + 1
		} else {
			return pmi.pagePrefix[mid] + (int)((addr-start)/uint64(pmi.pagesz))
		}
	}
	// pi := 0
	// for _, pme := range pmi.PagemapEntries {
	// 	pm := pme.Message.(*pagemap.PagemapEntry)
	// 	start := pm.GetVaddr()
	// 	n := pm.GetNrPages()
	// 	end := start + uint64(n*uint32(pmi.pagesz))
	// 	if addr >= start && addr < end {
	// 		m := (addr - start) / uint64(pmi.pagesz)
	// 		pi = pi + int(m)
	// 		return pi
	// 	}
	// 	pi += int(n)
	// }
	return -1
}

// Find page index for addr

func (pmi *TpagemapImg) find(addr uint64) int {
	pi := 0
	for _, pme := range pmi.PagemapEntries {
		pm := pme.Message.(*pagemap.PagemapEntry)
		start := pm.GetVaddr()
		n := pm.GetNrPages()
		end := start + uint64(n*uint32(pmi.pagesz))
		if addr >= start && addr < end {
			m := (addr - start) / uint64(pmi.pagesz)
			pi = pi + int(m)
			return pi
		}
		pi += int(n)
	}
	return -1
}

type Tmm struct {
	pagesz int
	*mm.MmEntry
}

func newTmm(imgdir, id string) (*Tmm, error) {
	img, err := ReadImg(imgdir, id, "mm")
	if err != nil {
		return nil, err
	}
	return &Tmm{
		pagesz:  os.Getpagesize(),
		MmEntry: img.Entries[0].Message.(*mm.MmEntry),
	}, nil
}

type Iov struct {
	pagesz    int
	start     uint64
	end       uint64
	copied    []bool
	img_start uint64 // XXX handle remaps
}

func newIov(pagesz int, start, end, img_start uint64) *Iov {
	return &Iov{
		pagesz:    pagesz,
		start:     start,
		end:       end,
		copied:    make([]bool, end-start, end-start), // XXX more compact representation?
		img_start: img_start,
	}
}

func (iov *Iov) String() string {
	return fmt.Sprintf("{[%x, %x) %d(%d) %x}", iov.start, iov.end, iov.end-iov.start, nPages(iov.start, iov.end, iov.pagesz), iov.img_start)
}

// Fetch max pages starting at addr0, but fewer if we run into a page
// that lazypagessrv already fetched.
func (iov *Iov) markFetchLen(addr0 uint64) int {
	max := 1 + PREFETCH
	n := 0
	addr := addr0
	for ; n < max && addr < iov.end; addr += uint64(iov.pagesz) {

		i := int(addr-iov.start) / iov.pagesz
		if iov.copied[i] {
			break
		}
		db.DPrintf(db.ALWAYS, "addr: %x end %x n:%d", addr, iov.end, n+1)
		iov.copied[i] = true
		n += 1
	}
	return n * iov.pagesz
}

// unmark pages
func (iov *Iov) unmarkFetchLen(addr0 uint64, n int) {
	addr := addr0
	for a := 0; a < n && addr < iov.end; addr += uint64(iov.pagesz) {
		i := int(addr-iov.start) / iov.pagesz
		iov.copied[i] = false
		a += 1
	}
	return
}

type Iovs struct {
	pagesz int
	iovs   []*Iov
}

func newIovs() *Iovs {
	return &Iovs{iovs: make([]*Iov, 0)}
}

func (iovs *Iovs) append(iov *Iov) {
	iovs.iovs = append(iovs.iovs, iov)
}

func (iovs *Iovs) len() int {
	return len(iovs.iovs)
}

func (iovs *Iovs) find(addr uint64) *Iov {
	for _, iov := range iovs.iovs {
		if iov.start <= addr && addr < iov.end {
			return iov
		}
	}
	return nil
}

func (iovs *Iovs) findBinSearch(addr uint64) int {
	//func (iovs *Iovs) findBinSearch(addr uint64) *Iov {
	low := 0
	high := len(iovs.iovs) - 1
	for low <= high {
		mid := (low + high) >> 1
		if iovs.iovs[mid].start > addr {
			high = mid - 1
		} else if iovs.iovs[mid].end <= addr {
			low = mid + 1
		} else {
			return mid
			//return iovs.iovs[mid]
		}
	}
	// for _, iov := range iovs.iovs {
	// 	if iov.start <= addr && addr < iov.end {
	// 		return iov
	// 	}
	// }
	return -1
}

// From criu/uffd.c: Create a list of IOVs that can be handled using
// userfaultfd. The IOVs generally correspond to lazy pagemap entries,
// except the cases when a single pagemap entry covers several
// VMAs. In those cases IOVs are split at VMA boundaries because
// UFFDIO_COPY may be done only inside a single VMA.  We assume here
// that pagemaps and VMAs are sorted.
func (mm *Tmm) collectIovs(pmi *TpagemapImg) (*Iovs, int, int) {
	db.DPrintf(db.TEST, "mmInfo %d pmes %d\n", len(mm.Vmas), len(pmi.PagemapEntries))

	iovs := newIovs()
	end := uint64(mm.pagesz)
	start := uint64(0)
	npages := uint32(0)
	maxIovLen := start
	nvma := 0
	db.DPrintf(db.CRIU, "FIRST entry: %v", pmi.PagemapEntries[0].Message.(*pagemap.PagemapEntry), end)
	for _, pme := range pmi.PagemapEntries[0:] {
		pm := pme.Message.(*pagemap.PagemapEntry)

		start = pm.GetVaddr()
		end = start + uint64(pm.GetNrPages()*uint32(mm.pagesz))
		npages += pm.GetNrPages()
		for ; nvma < len(mm.Vmas); nvma++ {
			vma := mm.Vmas[nvma]
			// if start >= vma.GetStart() {
			// 	continue
			// }
			if start >= vma.GetEnd() {
				continue
			}
			vend := vma.GetEnd()
			len := end
			if vend < end {
				len = vend
			}
			len = len - start
			iov := newIov(mm.pagesz, start, start+len, start)
			//db.DPrintf(db.ALWAYS, "iov start %x end %x vend %v", iov.start, iov.end, vend)
			iovs.append(iov)

			if len > maxIovLen {
				maxIovLen = len
			}

			if end < vend {
				break
			}
			start = vend
		}
	}
	db.DPrintf(db.TEST, "iovs %d\n", len(iovs.iovs))
	return iovs, int(npages), int(maxIovLen)
}

func (mm *Tmm) collectIovsbad(pmi *TpagemapImg) (*Iovs, int, int) {
	db.DPrintf(db.TEST, "mmInfo %d pmes %d\n", len(mm.Vmas), len(pmi.PagemapEntries))

	iovs := newIovs()
	//end := uint64(mm.pagesz)
	start := uint64(0)
	end := pmi.PagemapEntries[1].Message.(*pagemap.PagemapEntry).GetVaddr()
	npages := uint32(0)
	maxIovLen := start
	nvma := 0

	for i, pme := range pmi.PagemapEntries[1:] {
		pm := pme.Message.(*pagemap.PagemapEntry)
		if pm.GetVaddr() != end {
			len := end - start
			iov := newIov(mm.pagesz, start, start+len, start)
			//db.DPrintf(db.ALWAYS, "iov start %x end %x vend %v", iov.start, iov.end, vend)
			iovs.append(iov)

			if len > maxIovLen {
				maxIovLen = len
			}
			start = pm.GetVaddr()
		}
		end = pm.GetVaddr() + uint64(pm.GetNrPages()*uint32(mm.pagesz))
		npages += pm.GetNrPages()
		for ; nvma < len(mm.Vmas); nvma++ {
			vma := mm.Vmas[nvma]
			// if start >= vma.GetStart() {
			// 	continue
			// }
			if start >= vma.GetEnd() {
				continue
			}
			vend := vma.GetEnd()
			length := end
			if vend <= end {
				length = vend
			} else if i < int(len(pmi.PagemapEntries))-2 {
				break
			}
			length = length - start
			iov := newIov(mm.pagesz, start, start+length, start)
			db.DPrintf(db.ALWAYS, "iov start %x end %x vend %v", iov.start, iov.end, vend)
			iovs.append(iov)

			if length > maxIovLen {
				maxIovLen = length
			}

			if end < vend {
				break
			}
			start = vend
		}
	}
	db.DPrintf(db.TEST, "iovs %d\n", len(iovs.iovs))
	return iovs, int(npages), int(maxIovLen)
}

func FilterLazyPages(imgdir string) error {
	const (
		PE_LAZY uint32 = (1 << 1)
	)

	pmi, err := newTpagemapImg(imgdir, "1")
	if err != nil {
		return err
	}

	ph := pmi.PageMapHead.Message.(*pagemap.PagemapHead)
	pageId := int(ph.GetPagesId())

	pn := filepath.Join(imgdir, "pages-"+strconv.Itoa(pageId)+".img")
	src, err := os.Open(pn)
	if err != nil {
		return err
	}
	defer src.Close()

	pn = filepath.Join(imgdir, "pagesnonlazy-"+strconv.Itoa(pageId)+".img")
	dst, err := os.Create(pn)
	if err != nil {
		return err
	}
	defer dst.Close()

	page := make([]byte, pmi.pagesz)
	for _, pme := range pmi.PagemapEntries {
		pm := pme.Message.(*pagemap.PagemapEntry)
		n := pm.GetNrPages()
		for i := uint32(0); i < n; i++ {
			if _, err := src.Read(page); err != nil {
				return err
			}
			if pm.GetFlags()&PE_LAZY == PE_LAZY {
				// skip
			} else {
				if _, err := dst.Write(page); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func ExpandLazyPages(imgdir string) error {
	const (
		PE_LAZY uint32 = (1 << 1)
	)

	pmi, err := newTpagemapImg(imgdir, "1")
	if err != nil {
		return err
	}

	ph := pmi.PageMapHead.Message.(*pagemap.PagemapHead)
	pageId := int(ph.GetPagesId())

	pn := filepath.Join(imgdir, "pagesnonlazy-"+strconv.Itoa(pageId)+".img")
	src, err := os.Open(pn)
	if err != nil {
		return err
	}
	defer src.Close()

	pn = filepath.Join(imgdir, "pages-"+strconv.Itoa(pageId)+".img")
	dst, err := os.Create(pn)
	if err != nil {
		return err
	}
	defer dst.Close()

	page := make([]byte, pmi.pagesz)
	for _, pme := range pmi.PagemapEntries {
		pm := pme.Message.(*pagemap.PagemapEntry)
		n := pm.GetNrPages()
		for i := uint32(0); i < n; i++ {
			if pm.GetFlags()&PE_LAZY == PE_LAZY {
				if _, err := dst.Seek(int64(pmi.pagesz), 1); err != nil {
					return err
				}
			} else {
				if _, err := src.Read(page); err != nil {
					return err
				}
				if _, err := dst.Write(page); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

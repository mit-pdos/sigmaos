package lazypagessrv

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
	"github.com/checkpoint-restore/go-criu/v7/crit/images/mm"
	"github.com/checkpoint-restore/go-criu/v7/crit/images/pagemap"

	db "sigmaos/debug"
)

func readImg(imgdir string, pid int, magic string) (*crit.CriuImage, error) {
	pn := filepath.Join(imgdir, magic+"-"+strconv.Itoa(int(pid))+".img")
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

type TpagemapImg struct {
	pagesz         int
	PageMapHead    *crit.CriuEntry
	PagemapEntries []*crit.CriuEntry
}

func newTpagemapImg(imgdir string, pid int) (*TpagemapImg, error) {
	img, err := readImg(imgdir, pid, "pagemap")
	if err != nil {
		return nil, err
	}
	return &TpagemapImg{
			pagesz:         os.Getpagesize(),
			PageMapHead:    img.Entries[0],
			PagemapEntries: img.Entries[1:]},
		nil
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

func (pmi *TpagemapImg) readPage(pagesdir string, pid, pi int, page []byte) error {
	ph := pmi.PageMapHead.Message.(*pagemap.PagemapHead)
	pageId := int(ph.GetPagesId())
	pn := filepath.Join(pagesdir, "pages-"+strconv.Itoa(pageId)+".img")
	f, err := os.Open(pn)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Seek(int64(pi*pmi.pagesz), 0); err != nil {
		return err
	}
	if _, err := f.Read(page); err != nil {
		return err
	}
	return nil
}

type Tmm struct {
	pagesz int
	*mm.MmEntry
}

func newTmm(imgdir string, pid int) (*Tmm, error) {
	img, err := readImg(imgdir, pid, "mm")
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
	img_start uint64 // XXX handle remaps
}

func newIov(pagesz int, start, end, img_start uint64) *Iov {
	return &Iov{pagesz: pagesz, start: start, end: end, img_start: img_start}
}

func nPages(start, end uint64, pagesz int) int {
	len := end - start
	return int((len + uint64(pagesz) - 1) / uint64(pagesz))
}

func (iov *Iov) String() string {
	return fmt.Sprintf("{[%x, %x) %d(%d) %x}", iov.start, iov.end, iov.end-iov.start, nPages(iov.start, iov.end, iov.pagesz), iov.img_start)
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

// From criu/uffd.c: Create a list of IOVs that can be handled using
// userfaultfd. The IOVs generally correspond to lazy pagemap entries,
// except the cases when a single pagemap entry covers several
// VMAs. In those cases IOVs are split at VMA boundaries because
// UFFDIO_COPY may be done only inside a single VMA.  We assume here
// that pagemaps and VMAs are sorted.
func (mm *Tmm) collectIovs(pmi *TpagemapImg) (*Iovs, int, int) {
	db.DPrintf(db.TEST, "mmInfo %d\n", len(mm.Vmas))

	iovs := newIovs()
	end := uint64(mm.pagesz)
	start := uint64(0)
	npages := uint32(0)
	maxIovLen := start

	ph := pmi.PageMapHead.Message.(*pagemap.PagemapHead)
	db.DPrintf(db.TEST, "ph %v", ph)

	for _, pme := range pmi.PagemapEntries[1:] {
		pm := pme.Message.(*pagemap.PagemapEntry)

		start = pm.GetVaddr()
		end = start + uint64(pm.GetNrPages()*uint32(mm.pagesz))
		npages += pm.GetNrPages()

		for _, vma := range mm.Vmas {
			if start >= vma.GetStart() {
				continue
			}
			vend := vma.GetEnd()
			len := end
			if vend < end {
				end = vend
			}
			len = len - start
			iov := newIov(mm.pagesz, start, start+len, start)
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
	return iovs, int(npages), int(maxIovLen)
}

func FilterLazyPages(imgdir string, pid int) error {
	const (
		PE_LAZY uint32 = (1 << 1)
	)

	pmi, err := newTpagemapImg(imgdir, pid)
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

func ExpandLazyPages(imgdir string, pid int) error {
	const (
		PE_LAZY uint32 = (1 << 1)
	)

	pmi, err := newTpagemapImg(imgdir, pid)
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
	zero := make([]byte, pmi.pagesz)
	for _, pme := range pmi.PagemapEntries {
		pm := pme.Message.(*pagemap.PagemapEntry)
		n := pm.GetNrPages()
		for i := uint32(0); i < n; i++ {
			if pm.GetFlags()&PE_LAZY == PE_LAZY {
				if _, err := dst.Write(zero); err != nil {
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

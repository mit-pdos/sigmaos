package lazypagessrv

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
			db.DPrintf(db.ALWAYS, "m %d pi %d\n", m, pi)
			return pi
		}
		pi += int(n)
	}
	return -1
}

func (pmi *TpagemapImg) read(imgdir string, pid, pi int) ([]byte, error) {
	page := make([]byte, pmi.pagesz)
	if err := pmi.readPage(imgdir, pid, pi, page); err != nil {
		return nil, err
	}
	return page, nil
}

func (pmi *TpagemapImg) readPage(imgdir string, pid, pi int, page []byte) error {
	ph := pmi.PageMapHead.Message.(*pagemap.PagemapHead)
	pageId := int(ph.GetPagesId())
	pn := filepath.Join(imgdir, "pages-"+strconv.Itoa(pageId)+".img")
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
	start     uint64
	end       uint64
	img_start uint64
}

func (iov *Iov) String() string {
	return fmt.Sprintf("{%x %x %x}", iov.start, iov.end, iov.img_start)
}

type Iovs struct {
	iovs []*Iov
}

func newIovs() *Iovs {
	return &Iovs{iovs: make([]*Iov, 0)}
}

func (iovs *Iovs) append(iov *Iov) {
	iovs.iovs = append(iovs.iovs, iov)
}

func (iovs *Iovs) find(addr uint64) *Iov {
	for _, iov := range iovs.iovs {
		if iov.start <= addr && addr < iov.end {
			return iov
		}
	}
	return nil
}

func (mm *Tmm) collectIovs(pmi *TpagemapImg) *Iovs {
	db.DPrintf(db.TEST, "mmInfo %d\n", len(mm.Vmas))

	iovs := newIovs()
	end := uint64(mm.pagesz)
	start := uint64(0)
	nPages := uint32(0)
	max_iov_len := start

	ph := pmi.PageMapHead.Message.(*pagemap.PagemapHead)
	db.DPrintf(db.TEST, "ph %v", ph)

	for _, pme := range pmi.PagemapEntries[1:] {
		pm := pme.Message.(*pagemap.PagemapEntry)

		db.DPrintf(db.TEST, "pm %v", pm)

		start = pm.GetVaddr()
		end = start + uint64(pm.GetNrPages()*uint32(mm.pagesz))
		nPages += pm.GetNrPages()

		for _, vma := range mm.Vmas {
			if start >= vma.GetStart() {
				continue
			}
			iov := &Iov{}
			vend := vma.GetEnd()
			len := end
			if vend < end {
				end = vend
			}
			len = len - start
			iov.start = start
			iov.img_start = start
			iov.end = iov.start + len
			iovs.append(iov)

			if len > max_iov_len {
				max_iov_len = len
			}

			if end < vend {
				db.DPrintf(db.TEST, "%d vma %v\n", end, vma)
				break
			}
			start = vend
		}
	}
	// XXX do something with max_iov_len
	db.DPrintf(db.TEST, "max_iov_len %d\n", max_iov_len)
	return iovs
}

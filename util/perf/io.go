package perf

import (
	"io"
)

type PerfReader struct {
	rdr io.Reader
	p   *Perf
}

func NewPerfReader(rdr io.Reader, p *Perf) *PerfReader {
	return &PerfReader{
		rdr: rdr,
		p:   p,
	}
}

func (pr *PerfReader) Read(p []byte) (int, error) {
	n, err := pr.rdr.Read(p)
	pr.p.TptTick(float64(n))
	return n, err
}

type PerfWriter struct {
	wr io.Writer
	p  *Perf
}

func NewPerfWriter(wr io.Writer, p *Perf) *PerfWriter {
	return &PerfWriter{
		wr: wr,
		p:  p,
	}
}

func (pw *PerfWriter) Write(p []byte) (int, error) {
	n, err := pw.wr.Write(p)
	pw.p.TptTick(float64(n))
	return n, err
}

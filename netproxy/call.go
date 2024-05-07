package netproxy

import (
	"fmt"

	"sigmaos/sessp"
)

type ProxyCall struct {
	Seqno sessp.Tseqno
	Iov   sessp.IoVec
}

func NewProxyCall(seqno sessp.Tseqno, iov sessp.IoVec) *ProxyCall {
	return &ProxyCall{
		Seqno: seqno,
		Iov:   iov,
	}
}

func (pc *ProxyCall) Tag() sessp.Ttag {
	return sessp.Ttag(pc.Seqno)
}

func (pc *ProxyCall) String() string {
	return fmt.Sprintf("&{ NetProxyCall seqno:%v iov:%v }", pc.Seqno, pc.Iov)
}

package netsigma

import (
	"fmt"

	"sigmaos/sessp"
)

type ProxyCall struct {
	Seqno  sessp.Tseqno
	Iov    sessp.IoVec
	sendfd bool
}

func NewProxyCall(seqno sessp.Tseqno, iov sessp.IoVec, sendfd bool) *ProxyCall {
	return &ProxyCall{
		Seqno:  seqno,
		Iov:    iov,
		sendfd: sendfd,
	}
}

func (pc *ProxyCall) Tag() sessp.Ttag {
	return sessp.Ttag(pc.Seqno)
}

func (pc *ProxyCall) String() string {
	return fmt.Sprintf("&{ NetProxyCall seqno:%v iov:%v sendfd:%v }", pc.Seqno, pc.Iov, pc.sendfd)
}

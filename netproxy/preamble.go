package netproxy

import (
	"net"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/frame"
	sp "sigmaos/sigmap"
)

func writeConnPreamble(c net.Conn, p *sp.Tprincipal) error {
	// Marshal principal
	b, err := proto.Marshal(p)
	if err != nil {
		db.DFatalf("Error marshal principal: %v", err)
		return err
	}
	// Write the principal ID to the server's netproxyd, so that it
	// knows the principal associated with this connection
	if err := frame.WriteFrame(c, b); err != nil {
		db.DPrintf(db.ERROR, "Error WriteFrame principal preamble: %v", err)
		return err
	}
	return nil
}

func readConnPreamble(c net.Conn) (*sp.Tprincipal, error) {
	// Get the principal from the newly established connection
	b, err := frame.ReadFrame(c)
	if err != nil {
		db.DPrintf(db.NETSIGMA_ERR, "Error Read PrincipalID preamble frame: %v", err)
		return nil, err
	}
	p := sp.NoPrincipal()
	if err := proto.Unmarshal(b, p); err != nil {
		db.DPrintf(db.ERROR, "Error Unmarshal PrincipalID preamble: %v", err)
		return nil, err
	}
	return p, nil
}

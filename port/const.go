package port

import (
	sp "sigmaos/sigmap"
)

const (
	UPROCD_PORT       sp.Tport = 1112
	PUBLIC_HTTP_PORT           = UPROCD_PORT + 1
	PUBLIC_NAMED_PORT          = UPROCD_PORT + 2
)

package spin

import (
	db "sigmaos/debug"
)

// Consume some CPU by doing add & multiply ops.

func ConsumeCPU(niter int) {
	j := 0
	for i := 0; i < niter; i++ {
		j = j*i + i
	}
	db.DPrintf(db.NEVER, "%v", j)
}

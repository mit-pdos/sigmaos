package sigmap

import ()

type TleaseId uint64
type Tttl uint64

const NoLeaseId TleaseId = ^TleaseId(0)

type Lease interface {
	Lease() TleaseId
	Grant(ttl Tttl) (TleaseId, error)
	Refresh(TleaseId) error
}

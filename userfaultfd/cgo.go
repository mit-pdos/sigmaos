package userfaultfd

//
// extends github.com/loopholelabs/userfaultfd-go/pkg/constants
//

/*
#include <sys/syscall.h>
#include <fcntl.h>
#include <linux/userfaultfd.h>

struct uffd_pagefault {
	__u64	flags;
	__u64	address;
	__u32 ptid;
};
*/
import "C"
import "unsafe"

const (
	NR_userfaultfd = C.__NR_userfaultfd

	UFFD_API             = C.UFFD_API
	UFFD_EVENT_PAGEFAULT = C.UFFD_EVENT_PAGEFAULT

	UFFDIO_REGISTER_MODE_MISSING = C.UFFDIO_REGISTER_MODE_MISSING

	UFFDIO_API      = 3222841919 // From <linux/userfaultfd.h> macro
	UFFDIO_REGISTER = 3223366144 // From <linux/userfaultfd.h> macro
	UFFDIO_COPY     = 3223890435 // From <linux/userfaultfd.h> macro
	UFFDIO_ZEROPAGE = 3223366148 // From <linux/userfaultfd.h> macro
)

type (
	CULong = C.ulonglong
	CUChar = C.uchar
	CLong  = C.longlong

	UffdMsg       = C.struct_uffd_msg
	UffdPagefault = C.struct_uffd_pagefault

	UffdioAPI      = C.struct_uffdio_api
	UffdioRegister = C.struct_uffdio_register
	UffdioRange    = C.struct_uffdio_range
	UffdioCopy     = C.struct_uffdio_copy
	UffdioZeroPage = C.struct_uffdio_zeropage
)

func NewUffdioAPI(api, features CULong) UffdioAPI {
	return UffdioAPI{
		api:      api,
		features: features,
	}
}

func NewUffdioRegister(start, length, mode CULong) UffdioRegister {
	return UffdioRegister{
		_range: UffdioRange{
			start: start,
			len:   length,
		},
		mode: mode,
	}
}

func NewUffdioCopy(b []byte, address CULong, len CULong, mode CULong, copy CLong) UffdioCopy {
	return UffdioCopy{
		src:  CULong(uintptr(unsafe.Pointer(&b[0]))),
		dst:  address,
		len:  len,
		mode: mode,
		copy: copy,
	}
}

func NewUffdioZeroPage(start, length, mode CULong) UffdioZeroPage {
	return UffdioZeroPage{
		_range: UffdioRange{
			start: start,
			len:   length,
		},
		mode: mode,
	}
}

func GetMsgEvent(msg *UffdMsg) CUChar {
	return msg.event
}

func GetMsgArg(msg *UffdMsg) [24]byte {
	return msg.arg
}

func GetPagefaultAddress(pagefault *UffdPagefault) CULong {
	return pagefault.address
}

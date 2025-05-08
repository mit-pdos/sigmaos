#pragma once

#include <sigmap/types.h>

namespace sigmaos {
namespace sigmap::constants {

// Size constants
const sigmaos::sigmap::types::Tsize KBYTE = 1 << 10;
const sigmaos::sigmap::types::Tsize MBYTE = 1 << 20;
const sigmaos::sigmap::types::Tsize GBYTE = 1 << 30;
const sigmaos::sigmap::types::Tsize BUFSZ = 1 * MBYTE;
const sigmaos::sigmap::types::Tsize MAXGETSET Tsize = MBYTE // If need more than MaxGetSet, use Open/Read/Close interface

//// Size constants
//const (
//	KBYTE           = 1 << 10
//	MBYTE           = 1 << 20
//	GBYTE           = 1 << 30
//	BUFSZ           = 1 * MBYTE
//	MAXGETSET Tsize = MBYTE // If need more than MaxGetSet, use Open/Read/Close interface
//)
};
};

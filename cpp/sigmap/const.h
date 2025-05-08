#pragma once

#include <sigmap/types.h>

namespace sigmaos {
namespace sigmap::constants {

// Size constants
const sigmaos::sigmap::types::Tsize KBYTE = 1 << 10;
const sigmaos::sigmap::types::Tsize MBYTE = 1 << 20;
const sigmaos::sigmap::types::Tsize GBYTE = 1 << 30;
const sigmaos::sigmap::types::Tsize BUFSZ = 1 * MBYTE;
const sigmaos::sigmap::types::Tsize MAXGETSET = MBYTE; // If need more than MaxGetSet, use Open/Read/Close interface

// Protocol constants
const sigmaos::sigmap::types::Tfid NO_FID = ~0;
const sigmaos::sigmap::types::Tpath NO_PATH = ~0;
const sigmaos::sigmap::types::Toffset NO_OFFSET = ~0;
const sigmaos::sigmap::types::TclntID NO_CLNT_ID = ~0;
const sigmaos::sigmap::types::TleaseID NO_LEASE_ID = ~0;

};
};

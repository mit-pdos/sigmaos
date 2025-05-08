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

// Qid consts
// A Qid's type field represents the type of a file, the high 8 bits of
// the file's permission.
sigmaos::sigmap::types::TQversion NoV = ~0;
sigmaos::sigmap::types::Qtype QTDIR     = 0x80;
sigmaos::sigmap::types::Qtype QTAPPEND  = 0x40;
sigmaos::sigmap::types::Qtype QTEXCL    = 0x20;
sigmaos::sigmap::types::Qtype QTMOUNT   = 0x10;
sigmaos::sigmap::types::Qtype QTAUTH    = 0x08;
sigmaos::sigmap::types::Qtype QTTMP     = 0x04;
sigmaos::sigmap::types::Qtype QTSYMLINK = 0x02;
sigmaos::sigmap::types::Qtype QTFILE    = 0x00;

};
};

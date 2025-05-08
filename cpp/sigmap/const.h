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
const sigmaos::sigmap::types::TQversion NoV = ~0;
const sigmaos::sigmap::types::Qtype QTDIR     = 0x80;
const sigmaos::sigmap::types::Qtype QTAPPEND  = 0x40;
const sigmaos::sigmap::types::Qtype QTEXCL    = 0x20;
const sigmaos::sigmap::types::Qtype QTMOUNT   = 0x10;
const sigmaos::sigmap::types::Qtype QTAUTH    = 0x08;
const sigmaos::sigmap::types::Qtype QTTMP     = 0x04;
const sigmaos::sigmap::types::Qtype QTSYMLINK = 0x02;
const sigmaos::sigmap::types::Qtype QTFILE    = 0x00;

// Flags for the mode field in Topen and Tcreate messages
const sigmaos::sigmap::types::Tmode OREAD   = 0;    // read-only
const sigmaos::sigmap::types::Tmode OWRITE  = 0x01; // write-only
const sigmaos::sigmap::types::Tmode ORDWR   = 0x02; // read-write
const sigmaos::sigmap::types::Tmode OEXEC   = 0x03; // execute (implies OREAD)
const sigmaos::sigmap::types::Tmode OEXCL   = 0x04; // exclusive
const sigmaos::sigmap::types::Tmode OTRUNC  = 0x10; // or truncate file first
const sigmaos::sigmap::types::Tmode OCEXEC  = 0x20; // or close on exec
const sigmaos::sigmap::types::Tmode ORCLOSE = 0x40; // remove on close
const sigmaos::sigmap::types::Tmode OAPPEND = 0x80; // append

// Permissions
const sigmaos::sigmap::types::Tperm DMDIR    = 0x80000000; // directory
const sigmaos::sigmap::types::Tperm DMAPPEND = 0x40000000; // append only file
const sigmaos::sigmap::types::Tperm DMEXCL   = 0x20000000; // exclusive use file
const sigmaos::sigmap::types::Tperm DMMOUNT  = 0x10000000; // mounted channel
const sigmaos::sigmap::types::Tperm DMAUTH   = 0x08000000; // authentication file
const sigmaos::sigmap::types::Tperm DMTMP    = 0x04000000; // non-backed-up file

const sigmaos::sigmap::types::Tperm DMREAD  = 0x4; // mode bit for read permission
const sigmaos::sigmap::types::Tperm DMWRITE = 0x2; // mode bit for write permission
const sigmaos::sigmap::types::Tperm DMEXEC  = 0x1; // mode bit for execute permission

// 9P2000.u extensions
// A few are used by sigmaos, but not supported in driver/proxy,
// so sigmaos mounts on Linux without these extensions.
const sigmaos::sigmap::types::Tperm DMSYMLINK   = 0x02000000;
const sigmaos::sigmap::types::Tperm DMLINK      = 0x01000000;
const sigmaos::sigmap::types::Tperm DMDEVICE    = 0x00800000;
const sigmaos::sigmap::types::Tperm DMREPL      = 0x00400000;
const sigmaos::sigmap::types::Tperm DMNAMEDPIPE = 0x00200000;
const sigmaos::sigmap::types::Tperm DMSOCKET    = 0x00100000;
const sigmaos::sigmap::types::Tperm DMSETUID    = 0x00080000;
const sigmaos::sigmap::types::Tperm DMSETGID    = 0x00040000;
const sigmaos::sigmap::types::Tperm DMSETVTX    = 0x00010000;

// Generic constants
const std::string NOT_SET = "NOT_SET";

// FSETCD consts
const	sigmaos::sigmap::types::Tttl EtcdSessionTTL = 5;

// Realm consts
const	sigmaos::sigmap::types::Trealm ROOTREALM = "rootrealm";
const	sigmaos::sigmap::types::Trealm NO_REALM  = "NO_REALM";

// PID consts
const	sigmaos::sigmap::types::Tpid NO_PID = "no-pid";

// AWS Profile consts
const std::string AWS_PROFILE               = "sigmaos";
const std::string AWS_S3_RESTRICTED_PROFILE = "sigmaos-mr-restricted";

// Networking consts
const	sigmaos::sigmap::types::Tip NO_IP      = "";
const	sigmaos::sigmap::types::Tip LOCALHOST  = "127.0.0.1";
const	sigmaos::sigmap::types::Tport NO_PORT    = 0;

// Endpoint consts
const	sigmaos::sigmap::types::TTendpoint INTERNAL_EP = 1;
const	sigmaos::sigmap::types::TTendpoint EXTERNAL_EP = 2;

// Platform consts
const	sigmaos::sigmap::types::Tplatform PLATFORM_AWS       = "aws";
const	sigmaos::sigmap::types::Tplatform PLATFORM_CLOUDLAB  = "cloudlab";

// Auth consts
const	sigmaos::sigmap::types::TprincipalID NO_PRINCIPAL_ID  = "NO_PRINCIPAL_ID";

};
};

package sigmap

// Size constants
const (
	KBYTE           = 1 << 10
	MBYTE           = 1 << 20
	GBYTE           = 1 << 30
	BUFSZ           = 4 * MBYTE
	MAXGETSET Tsize = 1_000_000 // If need more than MaxGetSet, use Open/Read/Close interface
)

// Build constants
const (
	LOCAL_BUILD = "local-build"
)

// Protocol-level consts
const (
	// NoFid is a reserved fid used in a Tattach request for the afid
	// field, that indicates that the client does not wish to authenticate
	// this session.
	NoFid     Tfid     = ^Tfid(0)
	NoPath    Tpath    = ^Tpath(0)
	NoOffset  Toffset  = ^Toffset(0)
	NoClntId  TclntId  = ^TclntId(0)
	NoLeaseId TleaseId = ^TleaseId(0)
)

const (
	QTYPESHIFT = 24
	TYPESHIFT  = 16
	TYPEMASK   = 0xFF
)

// Qid consts
// A Qid's type field represents the type of a file, the high 8 bits of
// the file's permission.
const (
	NoV       TQversion = ^TQversion(0)
	QTDIR     Qtype     = 0x80 // directories
	QTAPPEND  Qtype     = 0x40 // append only files
	QTEXCL    Qtype     = 0x20 // exclusive use files
	QTMOUNT   Qtype     = 0x10 // mounted channel
	QTAUTH    Qtype     = 0x08 // authentication file (afid)
	QTTMP     Qtype     = 0x04 // non-backed-up file
	QTSYMLINK Qtype     = 0x02
	QTFILE    Qtype     = 0x00
)

// Flags for the mode field in Topen and Tcreate messages
const (
	OREAD   Tmode = 0    // read-only
	OWRITE  Tmode = 0x01 // write-only
	ORDWR   Tmode = 0x02 // read-write
	OEXEC   Tmode = 0x03 // execute (implies OREAD)
	OEXCL   Tmode = 0x04 // exclusive
	OTRUNC  Tmode = 0x10 // or truncate file first
	OCEXEC  Tmode = 0x20 // or close on exec
	ORCLOSE Tmode = 0x40 // remove on close
	OAPPEND Tmode = 0x80 // append
)

// Permissions
const (
	DMDIR    Tperm = 0x80000000 // directory
	DMAPPEND Tperm = 0x40000000 // append only file
	DMEXCL   Tperm = 0x20000000 // exclusive use file
	DMMOUNT  Tperm = 0x10000000 // mounted channel
	DMAUTH   Tperm = 0x08000000 // authentication file

	// DMTMP is ephemeral in sigmaP
	DMTMP Tperm = 0x04000000 // non-backed-up file

	DMREAD  = 0x4 // mode bit for read permission
	DMWRITE = 0x2 // mode bit for write permission
	DMEXEC  = 0x1 // mode bit for execute permission

	// 9P2000.u extensions
	// A few are used by sigmaos, but not supported in driver/proxy,
	// so sigmaos mounts on Linux without these extensions.
	DMSYMLINK   Tperm = 0x02000000
	DMLINK      Tperm = 0x01000000
	DMDEVICE    Tperm = 0x00800000
	DMREPL      Tperm = 0x00400000
	DMNAMEDPIPE Tperm = 0x00200000
	DMSOCKET    Tperm = 0x00100000
	DMSETUID    Tperm = 0x00080000
	DMSETGID    Tperm = 0x00040000
	DMSETVTX    Tperm = 0x00010000
)

// Generic consts
const (
	NOT_SET = "NOT_SET"
)

// FSETCD consts
const (
	EtcdSessionTTL = 5
)

// Path lookup consts
const (
	PATHCLNT_TIMEOUT  = 200 // ms  (XXX belongs in hyperparam?)
	PATHCLNT_MAXRETRY = (EtcdSessionTTL + 1) * (1000 / PATHCLNT_TIMEOUT)
)

// Realm consts
const (
	ROOTREALM Trealm = "rootrealm"
)

// PID consts
const (
	NO_PID Tpid = "no-pid"
)

// AWS Profile consts
const (
	AWS_PROFILE               = "sigmaos"
	AWS_S3_RESTRICTED_PROFILE = "sigmaos-mr-restricted"
)

// Networking consts
const (
	NO_IP              Tip     = ""
	LOCALHOST          Tip     = "127.0.0.1"
	INNER_CONTAINER_IP Tiptype = 1
	OUTER_CONTAINER_IP Tiptype = 2
	NO_PORT            Tport   = 0
)

// Auth consts
const (
	NO_PRINCIPAL_ID    TprincipalID = "NO_PRINCIPAL_ID"
	NO_REALM           Trealm       = "NO_REALM"
	KEY_LEN            int          = 256
	HOST_PRIV_KEY_FILE string       = "/tmp/sigmaos/master-key.priv"
	HOST_PUB_KEY_FILE  string       = "/tmp/sigmaos/master-key.pub"
	NO_SIGNER          string       = "NO_SIGNER"
	NO_SIGNED_TOKEN    string       = "NO_SIGNED_TOKEN"
)

var ALL_PATHS []string = []string{"*"}

func NoToken() *Ttoken {
	return &Ttoken{
		SignerStr:   NO_SIGNER,
		SignedToken: NO_SIGNED_TOKEN,
	}
}

func NoPrincipal() *Tprincipal {
	return &Tprincipal{
		IDStr:    NO_PRINCIPAL_ID.String(),
		RealmStr: NO_REALM.String(),
		Token:    NoToken(),
	}
}

#include <serr/serr.h>

namespace sigmaos {
namespace serr {

std::string TerrorToString(Terror e) {
	switch (e) {
	case TErrNoError:
		return "No error";
	case TErrBadattach:
		return "unknown specifier in attach";
	case TErrBadoffset:
		return "bad offset";
	case TErrBadcount:
		return "bad count";
	case TErrBotch:
		return "9P protocol botch";
	case TErrCreatenondir:
		return "create in non-directory";
	case TErrDupfid:
		return "duplicate fid";
	case TErrDuptag:
		return "duplicate tag";
	case TErrIsdir:
		return "is a directory";
	case TErrNocreate:
		return "create prohibited";
	case TErrNomem:
		return "out of memory";
	case TErrNoremove:
		return "remove prohibited";
	case TErrNostat:
		return "stat prohibited";
	case TErrNotfound:
		return "file not found";
	case TErrNowrite:
		return "write prohibited";
	case TErrNowstat:
		return "wstat prohibited";
	case TErrPerm:
		return "permission denied";
	case TErrUnknownfid:
		return "unknown fid";
	case TErrBaddir:
		return "bad directory in wstat";
	case TErrWalknodir:
		return "walk in non-directory";

	// sigma
	case TErrUnreachable:
		return "Unreachable";
	case TErrNotSupported:
		return "not supported";
	case TErrInval:
		return "invalid argument";
	case TErrUnknownMsg:
		return "unknown message";
	case TErrNotDir:
		return "not a directory";
	case TErrNotFile:
		return "not a file";
	case TErrNotSymlink:
		return "not a symlink";
	case TErrNotEmpty:
		return "not empty";
	case TErrVersion:
		return "version mismatch";
	case TErrStale:
		return "stale";
	case TErrExists:
		return "file exists";
	case TErrClosed:
		return "closed";
	case TErrBadFcall:
		return "bad fcall";

	// sigma OS errors
	case TErrRetry:
		return "retry";

	// for passing non-sigma errors through
	case TErrError:
		return "Non-sigma error";

	default:
		return "unknown error";
	}
}

};
};

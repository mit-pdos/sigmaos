#pragma once

#include <string>
#include <string_view>
#include <iostream>
#include <sstream>
#include <fmt/format.h>

namespace sigmaos {
namespace serr {

enum Terror {
	TErrNoError, 
	TErrBadattach,
	TErrBadoffset,
	TErrBadcount,
	TErrBotch,
	TErrCreatenondir,
	TErrDupfid,
	TErrDuptag,
	TErrIsdir,
	TErrNocreate,
	TErrNomem,
	TErrNoremove,
	TErrNostat,
	TErrNotfound,
	TErrNowrite,
	TErrNowstat,
	TErrPerm,
	TErrUnknownfid,
	TErrBaddir,
	TErrWalknodir,

	//
	// sigma protocol errors
	//

	TErrUnreachable,
	TErrNotSupported,
	TErrInval,
	TErrUnknownMsg,
	TErrNotDir,
	TErrNotFile,
	TErrNotSymlink,
	TErrNotEmpty,
	TErrVersion,
	TErrStale,
	TErrExists,
	TErrClosed, // for closed sessions and pipes.
	TErrBadFcall,

	//
	// sigma OS errors
	//

	TErrRetry, // tell client to retry

	//
	// To propagate non-sigma errors.
	// Must be *last* for String2Err()
	//
	TErrError,
};

std::string TerrorToString(Terror e);

class Error {
  public:
  Error(Terror err, std::string msg) : _err(err), _msg(msg) {}
  ~Error() {}

  Terror GetError() { return _err; }
  std::string GetMsg() { return _msg; }

  private:
	Terror _err;
	std::string _msg;
};

};
};

template<>
struct fmt::formatter<sigmaos::serr::Error>: fmt::formatter<string_view> {
  template<class ParseContext>
  constexpr ParseContext::iterator parse(ParseContext& ctx) {
    return ctx.end();
  }

  template<class FmtContext>
  FmtContext::iterator format(sigmaos::serr::Error e, FmtContext& ctx) const {
    std::ostringstream out;
    out << "{Err: \"" << sigmaos::serr::TerrorToString(e.GetError()) << "\" Obj: \"" << e.GetMsg() << "\" (<nil>)}";
    return std::ranges::copy(std::move(out).str(), ctx.out()).out;
  }
};



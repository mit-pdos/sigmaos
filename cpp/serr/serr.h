#pragma once

#include <string>
#include <string_view>
#include <iostream>
#include <sstream>
#include <fmt/format.h>

namespace sigmaos {
namespace serr {

enum Terror : int {
	TErrNoError = 0, 
	TErrBadattach = 1,
	TErrBadoffset = 2,
	TErrBadcount = 3,
	TErrBotch = 4,
	TErrCreatenondir = 5,
	TErrDupfid = 6,
	TErrDuptag = 7,
	TErrIsdir = 8,
  TErrNocreate = 9,
	TErrNomem = 10,
	TErrNoremove = 11,
	TErrNostat = 12,
	TErrNotfound = 13,
	TErrNowrite = 14,
	TErrNowstat = 15,
	TErrPerm = 16,
	TErrUnknownfid = 17,
	TErrBaddir = 18,
	TErrWalknodir = 19,

	//
	// sigma protocol errors
	//

	TErrUnreachable = 20,
	TErrNotSupported = 21,
	TErrInval = 22,
	TErrUnknownMsg = 23,
	TErrNotDir = 24,
	TErrNotFile = 25,
	TErrNotSymlink = 26,
	TErrNotEmpty = 27,
	TErrVersion = 28,
	TErrStale = 29,
	TErrExists = 30,
	TErrClosed = 31, // for closed sessions and pipes.
	TErrBadFcall = 32,
	TErrIO = 33,

	//
	// sigma OS errors
	//

	TErrRetry = 34, // tell client to retry

	//
	// To propagate non-sigma errors.
	// Must be *last* for String2Err()
	//
	TErrError = 35,
};

std::string TerrorToString(Terror e);

class Error {
  public:
  Error(Terror err, std::string msg) : _err(err), _msg(msg) {}
  ~Error() {}

  Terror GetError() { return _err; }
  std::string GetMsg() { return _msg; }
  std::string String() {
    std::ostringstream out;
    out << "{Err: \"" << sigmaos::serr::TerrorToString(_err) << "\" Obj: \"" << _msg << "\" (<nil>)}";
    return out.str();
  }

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

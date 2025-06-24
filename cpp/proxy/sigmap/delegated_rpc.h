#pragma once

#include <sys/socket.h>
#include <sys/un.h>
#include <unistd.h>

#include <iostream>
#include <memory>
#include <expected>

#include <google/protobuf/util/time_util.h>

#include <util/log/log.h>
#include <util/perf/perf.h>
#include <io/conn/conn.h>
#include <io/conn/unix/unix.h>
#include <io/transport/transport.h>
#include <io/demux/clnt.h>
#include <serr/serr.h>
#include <rpc/clnt.h>
#include <proc/proc.h>
#include <proc/status.h>
#include <sigmap/types.h>
#include <sigmap/const.h>
#include <sigmap/named.h>
#include <proxy/sigmap/sigmap.h>

namespace sigmaos {
namespace proxy::sigmap::delegated_rpc {

class Clnt {
  public:
  Clnt(std::shared_ptr<sigmaos::proxy::sigmap::Clnt> sp_clnt) : _sp_clnt(sp_clnt) {}
  ~Clnt() {}

  std::expected<int, sigmaos::serr::Error> RPC(uint64_t rpc_idx, google::protobuf::Message &rep);

  private:
  std::shared_ptr<sigmaos::proxy::sigmap::Clnt> _sp_clnt;
};

};
};

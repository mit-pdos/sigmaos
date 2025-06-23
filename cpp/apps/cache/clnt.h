#pragma once

#include <expected>
#include <atomic>

#include <google/protobuf/message.h>

#include <util/log/log.h>
#include <serr/serr.h>
#include <io/demux/clnt.h>
#include <io/iovec/iovec.h>
#include <rpc/spchannel/spchannel.h>
#include <rpc/clnt.h>
#include <proxy/sigmap/sigmap.h>

#include <util/tracing/proto/tracing.pb.h>
#include <apps/cache/proto/cache.pb.h>
#include <apps/cache/cache.h>

namespace sigmaos {
namespace apps::cache {

const std::string CACHECLNT = "CACHECLNT";
const std::string CACHECLNT_ERR = CACHECLNT + sigmaos::util::log::ERR;

// A channel/connection over which to make RPCs
class Clnt {
  public:
  Clnt(std::shared_ptr<sigmaos::proxy::sigmap::Clnt> sp_clnt, std::string srv_pn) : _srv_pn(srv_pn), _sp_clnt(sp_clnt) {}
  ~Clnt() {}
  // Initialize the channel
  std::expected<int, sigmaos::serr::Error> Init();
  std::expected<int, sigmaos::serr::Error> Get(std::string key, std::shared_ptr<std::string> val);
  std::expected<std::pair<std::vector<uint64_t>, std::shared_ptr<std::string>>, sigmaos::serr::Error> MultiGet(std::vector<std::string> keys);
  std::expected<int, sigmaos::serr::Error> Put(std::string key, std::shared_ptr<std::string> val);
  std::expected<int, sigmaos::serr::Error> Delete(std::string key);
  private:
  std::string _srv_pn;
  std::shared_ptr<sigmaos::proxy::sigmap::Clnt> _sp_clnt;
  std::shared_ptr<sigmaos::rpc::Clnt> _rpcc;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;
};

};
};

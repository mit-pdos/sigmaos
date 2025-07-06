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
  Clnt(std::shared_ptr<sigmaos::proxy::sigmap::Clnt> sp_clnt, std::string _svc_pn_base, uint32_t nsrv) : _mu(), _svc_pn_base(_svc_pn_base), _nsrv(nsrv), _sp_clnt(sp_clnt), _clnts() {}
  ~Clnt() {}
  std::expected<int, sigmaos::serr::Error> Get(std::string key, std::shared_ptr<std::string> val);
  std::expected<std::pair<std::vector<uint64_t>, std::shared_ptr<std::string>>, sigmaos::serr::Error> MultiGet(uint32_t srv_id, std::vector<std::string> &keys);
  std::expected<std::pair<std::vector<uint64_t>, std::shared_ptr<std::string>>, sigmaos::serr::Error> DelegatedMultiGet(uint64_t rpc_idx);
  std::expected<int, sigmaos::serr::Error> Put(std::string key, std::shared_ptr<std::string> val);
  std::expected<int, sigmaos::serr::Error> Delete(std::string key);
  private:
  std::mutex _mu;
  std::string _svc_pn_base;
  uint32_t _nsrv;
  std::shared_ptr<sigmaos::proxy::sigmap::Clnt> _sp_clnt;
  std::map<int, std::shared_ptr<sigmaos::rpc::Clnt>> _clnts;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;

  std::expected<std::shared_ptr<sigmaos::rpc::Clnt>, sigmaos::serr::Error> get_clnt(int srv_id, bool initialize);
};

};
};

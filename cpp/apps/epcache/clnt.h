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

#include <apps/epcache/proto/epcache.pb.h>

namespace sigmaos {
namespace apps::epcache {

const std::string EPCACHE_PN = "name/epcache";

const std::string EPCACHECLNT = "EPCACHECLNT";
const std::string EPCACHECLNT_ERR = EPCACHECLNT + sigmaos::util::log::ERR;

// A channel/connection over which to make RPCs
class Clnt {
  public:
  Clnt(std::shared_ptr<sigmaos::proxy::sigmap::Clnt> sp_clnt) : _sp_clnt(sp_clnt) {}
  ~Clnt() {}
  // Initialize the channel
  std::expected<int, sigmaos::serr::Error> Init();
  // Register an endpoint
  std::expected<int, sigmaos::serr::Error> RegisterEndpoint(std::string svc_name, std::string instance_id, std::shared_ptr<TendpointProto> ep);
  std::expected<int, sigmaos::serr::Error> DeregisterEndpoint(std::string svc_name, std::string instance_id);
//  std::expected<std::pair<std::vector<std::shared_ptr<Instance>>, Tversion>, sigmaos::serr::Error> DeregisterEndpoint(std::string svc_name, std::string instance_id);
  private:
  std::shared_ptr<sigmaos::proxy::sigmap::Clnt> _sp_clnt;
  std::shared_ptr<sigmaos::rpc::Clnt> _rpcc;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;
};

};
};

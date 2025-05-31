#include <apps/epcache/clnt.h>

namespace sigmaos {
namespace apps::epcache {

bool Clnt::_l = sigmaos::util::log::init_logger(EPCACHECLNT);
bool Clnt::_l_e = sigmaos::util::log::init_logger(EPCACHECLNT_ERR);

// Initialize the channel & connect to the server
std::expected<int, sigmaos::serr::Error> Clnt::Init() {
  {
    // Create a sigmap RPC channel to the server via the sigmaproxy
    log(EPCACHECLNT, "Create channel");
    auto chan = std::make_shared<sigmaos::rpc::spchannel::Channel>(EPCACHE_PN, _sp_clnt);
    // Initialize the channel
    auto res = chan->Init();
    if (!res.has_value()) {
      log(EPCACHECLNT_ERR, "Error create channel: {}", res.error().String());
      return std::unexpected(res.error());
    }
    log(EPCACHECLNT, "Create RPC client");
    // Create an RPC client from the channel
    _rpcc = std::make_shared<sigmaos::rpc::Clnt>(chan);
  }
  log(EPCACHECLNT, "Init successful!");
  return 0;
}

std::expected<int, sigmaos::serr::Error> Clnt::RegisterEndpoint(std::string svc_name, std::string instance_id, std::shared_ptr<TendpointProto> ep) {
	log(EPCACHECLNT, "RegisterEndpoint: {} -> {}", svc_name, ep->ShortDebugString());
  Instance i;
  i.set_id(instance_id);
  i.set_allocated_endpointproto(ep.get());
	RegisterEndpointRep rep;
	RegisterEndpointReq req;
  req.set_servicename(svc_name);
  req.set_allocated_instance(&i);
	{
    auto res = _rpcc->RPC("EPCacheSrv.RegisterEndpoint", req, rep);
    {
      auto _ = i.release_endpointproto();
    }
    {
      auto _ = req.release_instance();
    }
    if (!res.has_value()) {
      log(EPCACHECLNT_ERR, "Error RegisterEndpoint: {}", res.error().String());
      return std::unexpected(res.error());
    }
  }
	if (!rep.ok()) {
    log(EPCACHECLNT_ERR, "RegisterEndpoint failed");
    return std::unexpected(sigmaos::serr::Error(sigmaos::serr::TErrError, "Registration failed"));
	}
	log(EPCACHECLNT, "RegisterEndpoint ok: {} -> {}", svc_name, ep->ShortDebugString());
  return 0;
}
std::expected<int, sigmaos::serr::Error> Clnt::DeregisterEndpoint(std::string svc_name, std::string instance_id) {
	log(EPCACHECLNT, "DeregisterEndpoint: {} -> {}", svc_name, instance_id);
	DeregisterEndpointRep rep;
	DeregisterEndpointReq req;
  req.set_servicename(svc_name);
  req.set_instanceid(instance_id);
  {
	  auto res = _rpcc->RPC("EPCacheSrv.DeregisterEndpoint", req, &rep);
    if (!res.has_value()) {
      log(EPCACHECLNT_ERR, "Error DergisterEndpoint: {}", res.error().String());
      return std::unexpected(res.error());
    }
  }
	if (!rep.ok()) {
    log(EPCACHECLNT_ERR, "DeregisterEndpoint failed");
    return std::unexpected(sigmaos::serr::Error(sigmaos::serr::TErrError, "Deregistration failed"));
	}
	log(EPCACHECLNT, "DeregisterEndpoint ok: {} -> {}", svc_name, instance_id);
  return 0;
}

//  std::expected<std::pair<std::vector<std::shared_ptr<Instance>>, Tversion>, sigmaos::serr::Error> DeregisterEndpoint(std::string svc_name, std::string instance_id);

};
};

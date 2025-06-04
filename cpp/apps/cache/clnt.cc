#include <apps/cache/clnt.h>

namespace sigmaos {
namespace apps::cache {

bool Clnt::_l = sigmaos::util::log::init_logger(CACHECLNT);
bool Clnt::_l_e = sigmaos::util::log::init_logger(CACHECLNT_ERR);

// Initialize the channel & connect to the server
std::expected<int, sigmaos::serr::Error> Clnt::Init() {
  {
    // Create a sigmap RPC channel to the server via the sigmaproxy
    log(CACHECLNT, "Create channel");
    auto chan = std::make_shared<sigmaos::rpc::spchannel::Channel>(_srv_pn, _sp_clnt);
    // Initialize the channel
    auto res = chan->Init();
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error create channel: {}", res.error().String());
      return std::unexpected(res.error());
    }
    log(CACHECLNT, "Create RPC client");
    // Create an RPC client from the channel
    _rpcc = std::make_shared<sigmaos::rpc::Clnt>(chan);
  }
  log(CACHECLNT, "Init successful!");
  return 0;
}

std::expected<int, sigmaos::serr::Error> Clnt::Get(std::string key, std::string *val) {
	log(CACHECLNT, "Get: {}", key);
	CacheRep rep;
  CacheReq req;
  req.set_key(key);
  req.set_shard(key2shard(key));
	{
    auto res = _rpcc->RPC("CacheSrv.Get", req, rep);

    *val = rep.value();
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error Get: {}", res.error().String());
      return std::unexpected(res.error());
    }
  }
	log(CACHECLNT, "Get ok: {} -> {}b", key, val->size());
  return 0;
}

std::expected<int, sigmaos::serr::Error> Clnt::Put(std::string key, std::string *val) {
	log(CACHECLNT, "Put: {} -> {}b", key, val->size());
	CacheRep rep;
	CacheReq req;
  req.set_key(key);
  req.set_shard(key2shard(key));
  req.set_allocated_value(val);
	{
    auto res = _rpcc->RPC("CacheSrv.Put", req, rep);
    {
      auto _ = req.release_value();
    }
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error Put: {}", res.error().String());
      return std::unexpected(res.error());
    }
  }
	log(CACHECLNT, "Put ok: {} -> {}b", key, val->size());
  return 0;
}

std::expected<int, sigmaos::serr::Error> Clnt::Delete(std::string key) {
	log(CACHECLNT, "Delete: {}", key);
	CacheRep rep;
	CacheReq req;
  req.set_key(key);
  req.set_shard(key2shard(key));
	{
    auto res = _rpcc->RPC("CacheSrv.Delete", req, rep);
    {
      auto _ = req.release_value();
    }
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error Delete: {}", res.error().String());
      return std::unexpected(res.error());
    }
  }
	log(CACHECLNT, "Delete ok: {}", key);
  return 0;
}

};
};

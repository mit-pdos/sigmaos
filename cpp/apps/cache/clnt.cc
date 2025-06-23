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

std::expected<int, sigmaos::serr::Error> Clnt::Get(std::string key, std::shared_ptr<std::string> val) {
	log(CACHECLNT, "Get: {}", key);
  TfenceProto fence;
	CacheRep rep;
  CacheReq req;
  req.set_allocated_fence(&fence);
  req.set_key(key);
  req.set_shard(key2shard(key));
	{
    auto res = _rpcc->RPC("CacheSrv.Get", req, rep);
    {
      auto _ = req.release_fence();
    }
    auto start = GetCurrentTime();
    *val = rep.value();
    log(PROXY_RPC_LAT, "Set val to reply value lat:{}ms", LatencyMS(start));
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error Get: {}", res.error().String());
      return std::unexpected(res.error());
    }
  }
	log(CACHECLNT, "Get ok: {} -> {}b", key, val->size());
  return 0;
}

std::expected<std::pair<std::vector<uint64_t>, std::shared_ptr<std::string>>, sigmaos::serr::Error> Clnt::MultiGet(std::vector<std::string> keys) {
	log(CACHECLNT, "MultiGet nkey {}", keys.size());
  TfenceProto fence;
	CacheMultiGetRep rep;
  CacheMultiGetReq req;
  req.set_allocated_fence(&fence);
  Blob blob;
  auto iov = blob.mutable_iov();
  // Add a buffer to hold the output
  auto buf = std::make_shared<std::string>();
  iov->AddAllocated(buf.get());
  auto gets = req.mutable_gets();
  for (int i = 0; i < keys.size(); i++) {
    auto get = gets->Add();
    get->set_key(keys.at(i));
    get->set_shard(key2shard(keys.at(i)));
  }
  rep.set_allocated_blob(&blob);
	{
    auto res = _rpcc->RPC("CacheSrv.MultiGet", req, rep);
    {
      auto _ = req.release_fence();
    }
    if (!res.has_value()) {
      log(CACHECLNT_ERR, "Error Get: {}", res.error().String());
      return std::unexpected(res.error());
    }
  }
  std::vector<uint64_t> lengths(rep.lengths().size(), 0);
  for (int i = 0; i < lengths.size(); i++) {
    lengths[i] = rep.lengths().at(i);
  }
	log(CACHECLNT, "Get ok");
  return std::make_pair(lengths, buf);
}

std::expected<int, sigmaos::serr::Error> Clnt::Put(std::string key, std::shared_ptr<std::string> val) {
	log(CACHECLNT, "Put: {} -> {}b", key, val->size());
  TfenceProto fence;
	CacheRep rep;
	CacheReq req;
  req.set_allocated_fence(&fence);
  req.set_key(key);
  req.set_shard(key2shard(key));
  req.set_allocated_value(val.get());
	{
    auto res = _rpcc->RPC("CacheSrv.Put", req, rep);
    {
      auto _ = req.release_value();
    }
    {
      auto _ = req.release_fence();
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
  TfenceProto fence;
	CacheRep rep;
	CacheReq req;
  req.set_allocated_fence(&fence);
  req.set_key(key);
  req.set_shard(key2shard(key));
	{
    auto res = _rpcc->RPC("CacheSrv.Delete", req, rep);
    {
      auto _ = req.release_value();
    }
    {
      auto _ = req.release_fence();
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

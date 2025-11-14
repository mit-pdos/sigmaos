#pragma once

#include <apps/cache/cache.h>
#include <apps/cache/proto/cache.pb.h>
#include <apps/cache/proto/get.pb.h>
#include <apps/cache/proto/dump.pb.h>
#include <apps/cache/shard.h>
#include <google/protobuf/message.h>
#include <io/demux/clnt.h>
#include <io/iovec/iovec.h>
#include <proxy/sigmap/sigmap.h>
#include <rpc/clnt.h>
#include <rpc/rpc.h>
#include <rpc/spchannel/spchannel.h>
#include <serr/serr.h>
#include <shmem/shmem.h>
#include <util/log/log.h>
#include <util/tracing/proto/tracing.pb.h>

#include <atomic>
#include <expected>

namespace sigmaos {
namespace apps::cache {

const std::string CACHECLNT = "CACHECLNT";
const std::string CACHECLNT_ERR = CACHECLNT + sigmaos::util::log::ERR;

// A channel/connection over which to make RPCs
class Clnt {
 public:
  Clnt(std::shared_ptr<sigmaos::proxy::sigmap::Clnt> sp_clnt,
       std::string _svc_pn_base, uint32_t nsrv)
      : _mu(),
        _svc_pn_base(_svc_pn_base),
        _nsrv(nsrv),
        _sp_clnt(sp_clnt),
        _clnts() {}
  ~Clnt() {}
  std::expected<int, sigmaos::serr::Error> InitClnts(uint32_t last_srv_id);
  std::expected<int, sigmaos::serr::Error> InitClnt(uint32_t srv_id);
  std::expected<int, sigmaos::serr::Error> Get(
      std::string key, std::shared_ptr<std::string> val);
  std::expected<std::shared_ptr<
                    std::vector<std::shared_ptr<sigmaos::apps::cache::Value>>>,
                sigmaos::serr::Error>
  MultiGet(uint32_t srv_id, std::vector<std::string> &keys);
  std::expected<std::shared_ptr<
                    std::vector<std::shared_ptr<sigmaos::apps::cache::Value>>>,
                sigmaos::serr::Error>
  DelegatedMultiGet(uint64_t rpc_idx);
  std::expected<int, sigmaos::serr::Error> Put(
      std::string key, std::shared_ptr<std::string> val);
  std::expected<int, sigmaos::serr::Error> Delete(std::string key);
  std::expected<
      std::shared_ptr<std::map<std::string, std::shared_ptr<std::string>>>,
      sigmaos::serr::Error>
  DumpShard(uint32_t shard, bool empty);
  std::expected<
      std::shared_ptr<std::map<
          uint32_t,
          std::shared_ptr<std::map<
              std::string, std::shared_ptr<sigmaos::apps::cache::Value>>>>>,
      sigmaos::serr::Error>
  MultiDumpShard(uint32_t srv, std::vector<uint32_t> &shards);
  std::expected<
      std::shared_ptr<std::map<
          uint32_t,
          std::shared_ptr<std::map<
              std::string, std::shared_ptr<sigmaos::apps::cache::Value>>>>>,
      sigmaos::serr::Error>
  DelegatedMultiDumpShard(uint64_t rpc_idx, std::vector<uint32_t> &shards);
  std::expected<
      std::shared_ptr<std::map<std::string, std::shared_ptr<std::string>>>,
      sigmaos::serr::Error>
  DelegatedDumpShard(uint64_t rpc_idx);
  std::expected<int, sigmaos::serr::Error> BatchFetchDelegatedRPCs(
      std::vector<uint64_t> &rpc_idxs, int n_iov);

 private:
  std::mutex _mu;
  std::string _svc_pn_base;
  uint32_t _nsrv;
  std::shared_ptr<sigmaos::proxy::sigmap::Clnt> _sp_clnt;
  std::map<int, std::shared_ptr<sigmaos::rpc::Clnt>> _clnts;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;

  std::expected<std::shared_ptr<sigmaos::rpc::Clnt>, sigmaos::serr::Error>
  get_clnt(int srv_id, bool initialize);
  void init_clnt(
      std::shared_ptr<std::promise<std::expected<int, sigmaos::serr::Error>>>
          result,
      uint32_t srv_id);
};

};  // namespace apps::cache
};  // namespace sigmaos

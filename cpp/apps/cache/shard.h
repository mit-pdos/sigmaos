#pragma once

#include <apps/cache/clnt.h>
#include <io/conn/conn.h>
#include <io/conn/tcp/tcp.h>
#include <io/demux/srv.h>
#include <io/net/srv.h>
#include <io/transport/transport.h>
#include <proxy/sigmap/sigmap.h>
#include <rpc/srv.h>
#include <serr/serr.h>
#include <sigmap/const.h>
#include <sigmap/sigmap.pb.h>
#include <util/log/log.h>
#include <util/perf/perf.h>

#include <cmath>
#include <expected>
#include <filesystem>
#include <format>
#include <future>
#include <limits>
#include <memory>
#include <vector>

namespace sigmaos {
namespace apps::cache {

class Shard {
 public:
  Shard() : _mu(), _map(), _hit_cnt(0), _old_hit_cnt(0) {}
  ~Shard() {}

  std::expected<std::shared_ptr<std::string>, sigmaos::serr::Error> Get(
      std::string &key);
  void Put(std::string &key, std::shared_ptr<std::string> val);
  bool Delete(std::string &key);
  void Fill(std::map<std::string, std::string> kvs);

 private:
  std::mutex _mu;
  std::map<std::string, std::shared_ptr<std::string>> _map;
  uint64_t _hit_cnt;
  uint64_t _old_hit_cnt;
};

};  // namespace apps::cache
};  // namespace sigmaos

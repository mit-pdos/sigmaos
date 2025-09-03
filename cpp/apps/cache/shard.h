#pragma once

#include <memory>
#include <vector>
#include <expected>
#include <format>
#include <filesystem>
#include <limits>
#include <future>
#include <cmath>

#include <util/log/log.h>
#include <util/perf/perf.h>
#include <io/net/srv.h>
#include <io/conn/conn.h>
#include <io/transport/transport.h>
#include <io/conn/tcp/tcp.h>
#include <io/demux/srv.h>
#include <serr/serr.h>
#include <sigmap/sigmap.pb.h>
#include <sigmap/const.h>
#include <rpc/srv.h>
#include <proxy/sigmap/sigmap.h>
#include <apps/cache/clnt.h>

namespace sigmaos {
namespace apps::cache {

class Shard {
  public:
  Shard() : _mu(), _map(), _hit_cnt(0), _old_hit_cnt(0) {}
  ~Shard() {}

  std::expected<std::shared_ptr<std::string>, sigmaos::serr::Error> Get(std::string &key);
  void Put(std::string &key, std::shared_ptr<std::string> val);
  bool Delete(std::string &key);
  void Fill(std::map<std::string, std::string> kvs);

  private:
  std::mutex _mu;
  std::map<std::string, std::shared_ptr<std::string>> _map;
  uint64_t _hit_cnt;
  uint64_t _old_hit_cnt;
};

};
};

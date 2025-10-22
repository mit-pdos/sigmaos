#pragma once

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

class Value {
 public:
  Value(std::shared_ptr<std::string> unique_buf)
      : _off(0),
        _len(unique_buf->size()),
        _view_buf(nullptr),
        _unique_buf(unique_buf),
        _shared_buf(nullptr) {}
  Value(std::shared_ptr<std::string_view> view_buf, int off, int len)
      : _off(off),
        _len(len),
        _view_buf(view_buf),
        _unique_buf(nullptr),
        _shared_buf(nullptr) {}
  Value(std::shared_ptr<std::string> shared_buf, int off, int len)
      : _off(off),
        _len(len),
        _view_buf(nullptr),
        _unique_buf(nullptr),
        _shared_buf(shared_buf) {}
  ~Value() {}

  std::shared_ptr<std::string> Get();
  std::shared_ptr<std::string_view> GetStringView();

 private:
  int _off;
  int _len;
  std::shared_ptr<std::string_view>
      _view_buf;  // If constructed from a buffer whose memory is not owned by
                  // the C++ allocator (e.g., the buffer lives in shared memory)
  std::shared_ptr<std::string>
      _unique_buf;  // If constructed from a unique buffer (e.g., a Put), or
                    // copied from a shared buffer, store the unique buffer here
  std::shared_ptr<std::string>
      _shared_buf;  // If constructed from a shared buffer, store a reference to
                    // the shared buffer here
};

class Shard {
 public:
  Shard() : _mu(), _map(), _hit_cnt(0), _old_hit_cnt(0) {}
  ~Shard() {}

  std::expected<std::shared_ptr<std::string>, sigmaos::serr::Error> Get(
      std::string &key);
  void Put(std::string &key, std::shared_ptr<std::string> val);
  bool Delete(std::string &key);
  void Fill(std::shared_ptr<std::map<std::string, std::shared_ptr<Value>>> kvs);

 private:
  std::mutex _mu;
  std::map<std::string, std::shared_ptr<Value>> _map;
  uint64_t _hit_cnt;
  uint64_t _old_hit_cnt;
};

};  // namespace apps::cache
};  // namespace sigmaos

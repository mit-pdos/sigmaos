#pragma once

#include <apps/cache/clnt.h>
#include <apps/cache/shard.h>
#include <apps/cossim/proto/cossim.pb.h>
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
#include <limits>
#include <memory>
#include <vector>

namespace sigmaos {
namespace apps::cossim {

class Vector {
 public:
  Vector(::Vector *proto_vec, int dim)
      : _proto_vec(proto_vec),
        _underlying_buf(nullptr),
        _vals(nullptr),
        _dim(dim) {}
  Vector(std::shared_ptr<std::string> underlying_buf, char *vals, int dim)
      : _proto_vec(nullptr),
        _underlying_buf(underlying_buf),
        _vals((double *)vals),
        _dim(dim) {}
  Vector(std::shared_ptr<sigmaos::apps::cache::Value> underlying_val, int dim)
      : _proto_vec(nullptr),
        _underlying_buf(nullptr),
        _underlying_val(underlying_val),
        _vals((double *)underlying_val->GetStringView()->data()),
        _dim(dim) {}
  ~Vector() {}

  double Get(int idx) const;
  double CosineSimilarity(std::shared_ptr<Vector> other) const;

 private:
  ::Vector *_proto_vec;
  std::shared_ptr<std::string> _underlying_buf;
  std::shared_ptr<sigmaos::apps::cache::Value> _underlying_val;
  double *_vals;
  int _dim;
};

};  // namespace apps::cossim
};  // namespace sigmaos

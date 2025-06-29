#pragma once

#include <memory>
#include <vector>
#include <expected>
#include <format>
#include <filesystem>
#include <limits>
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
#include <apps/cossim/proto/cossim.pb.h>

namespace sigmaos {
namespace apps::cossim {

class Vector {
  public:
  Vector(::Vector *proto_vec, int dim) : _proto_vec(proto_vec), _underlying_buf(nullptr), _vals(nullptr), _dim(dim) {}
  Vector(std::shared_ptr<std::string> underlying_buf, char *vals, int dim) : _proto_vec(nullptr), _underlying_buf(underlying_buf), _vals((double *) vals), _dim(dim) {}
  ~Vector() {}

  double Get(int idx) const;
  double CosineSimilarity(std::shared_ptr<Vector> other) const;

  private:
  ::Vector *_proto_vec;
  std::shared_ptr<std::string> _underlying_buf;
  double *_vals;
  int _dim;
};

};
};

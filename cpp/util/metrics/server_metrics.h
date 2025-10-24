#pragma once

#include <atomic>

namespace sigmaos {
namespace util::metrics {

class ServerMetrics {
 public:
  ServerMetrics() : _n_req_in_flight(0) {}
  ~ServerMetrics() {}

  void StartRequest();
  void EndRequest();
  int GetNRequestsInFlight() const;

 private:
  std::atomic<int> _n_req_in_flight;
};

};  // namespace util::metrics
};  // namespace sigmaos

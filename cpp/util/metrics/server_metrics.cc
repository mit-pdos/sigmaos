#include <util/metrics/server_metrics.h>

namespace sigmaos {
namespace util::metrics {

void ServerMetrics::StartRequest() {
  _n_req_in_flight.fetch_add(1, std::memory_order_relaxed);
}

void ServerMetrics::EndRequest() {
  _n_req_in_flight.fetch_sub(1, std::memory_order_relaxed);
}

int ServerMetrics::GetNRequestsInFlight() const {
  return _n_req_in_flight.load(std::memory_order_relaxed);
}

};  // namespace util::metrics
};  // namespace sigmaos

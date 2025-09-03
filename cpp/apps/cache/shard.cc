#include <apps/cache/shard.h>

namespace sigmaos {
namespace apps::cache {

std::expected<std::shared_ptr<std::string>, sigmaos::serr::Error> Shard::Get(
    std::string &key) {
  std::lock_guard<std::mutex> guard(_mu);

  if (!_map.contains(key)) {
    return std::unexpected(sigmaos::serr::Error(
        sigmaos::serr::Terror::TErrNotfound, std::format("key {}", key)));
  }
  return _map.at(key);
}

void Shard::Put(std::string &key, std::shared_ptr<std::string> val) {
  std::lock_guard<std::mutex> guard(_mu);
  _map.at(key) = val;
}

bool Shard::Delete(std::string &key) {
  std::lock_guard<std::mutex> guard(_mu);
  bool existed = _map.contains(key);
  _map.erase(key);
  return existed;
}

void Shard::Fill(std::map<std::string, std::string> kvs) {
  std::lock_guard<std::mutex> guard(_mu);
  for (auto &[k, v] : kvs) {
    _map.at(k) = std::make_shared<std::string>(v);
  }
}

};  // namespace apps::cache
};  // namespace sigmaos

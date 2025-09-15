#include <apps/cache/shard.h>

namespace sigmaos {
namespace apps::cache {

std::shared_ptr<std::string> Value::Get() {
  // If constructed from a shared buffer, but this is the first get, copy the
  // data into a unique buffer, and release the underlying shared buffer.
  if (_shared_buf && !_unique_buf) {
    _unique_buf = std::make_shared<std::string>(*_shared_buf, _off, _len);
    _shared_buf = nullptr;
  }
  return _unique_buf;
}

std::expected<std::shared_ptr<std::string>, sigmaos::serr::Error> Shard::Get(
    std::string &key) {
  std::lock_guard<std::mutex> guard(_mu);

  if (!_map.contains(key)) {
    return std::unexpected(sigmaos::serr::Error(
        sigmaos::serr::Terror::TErrNotfound, std::format("key {}", key)));
  }
  auto v = _map.at(key);
  return v->Get();
}

void Shard::Put(std::string &key, std::shared_ptr<std::string> val) {
  std::lock_guard<std::mutex> guard(_mu);
  _map.at(key) = std::make_shared<Value>(val);
}

bool Shard::Delete(std::string &key) {
  std::lock_guard<std::mutex> guard(_mu);
  bool existed = _map.contains(key);
  _map.erase(key);
  return existed;
}

void Shard::Fill(
    std::shared_ptr<std::map<std::string, std::shared_ptr<Value>>> kvs) {
  std::lock_guard<std::mutex> guard(_mu);
  for (auto &[k, v] : *kvs) {
    _map[k] = v;
  }
}

};  // namespace apps::cache
};  // namespace sigmaos

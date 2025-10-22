#include <apps/cache/shard.h>

namespace sigmaos {
namespace apps::cache {

std::shared_ptr<std::string> Value::Get() {
  // If constructed from an underlying buffer whose memory isn't owned by the
  // C++ allocator (e.g., a shared memory segment), but this is the first get,
  // copy the data into a unique buffer, and release the underlying shared
  // buffer.
  if (_view_buf && !_unique_buf) {
    _unique_buf = std::make_shared<std::string>(*_view_buf, _off, _len);
    _view_buf = nullptr;
  } else {
    // If constructed from a shared buffer, but this is the first get, copy the
    // data into a unique buffer, and release the underlying shared buffer.
    if (_shared_buf && !_unique_buf) {
      _unique_buf = std::make_shared<std::string>(*_shared_buf, _off, _len);
      _shared_buf = nullptr;
    }
  }
  return _unique_buf;
}

std::shared_ptr<std::string_view> Value::GetStringView() {
  // Sanity check
  if (!_view_buf && !_shared_buf) {
    fatal("Get String view from non-shared-buf-backed cache value");
  }
  if (_shared_buf) {
    return std::make_shared<std::string_view>(_shared_buf->data() + _off, _len);
  }
  return std::make_shared<std::string_view>(_view_buf->data() + _off, _len);
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

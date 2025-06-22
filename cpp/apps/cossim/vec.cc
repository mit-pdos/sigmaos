#include <apps/cossim/vec.h>

namespace sigmaos {
namespace apps::cossim {

double Vector::Get(int idx) const {
  if (idx < 0 || idx > _dim) {
    fatal("idx {} out of bounds for vector with length {}", idx, _dim);
  }
  if (_proto_vec) {
    return _proto_vec->vals()[idx];
  }
  return _vals[idx];
}

double Vector::CosineSimilarity(std::shared_ptr<Vector> other) const {
  // Compute cosine similarity
  double other_l2 = 0.0;
  double vec_l2 = 0.0;
  double cos_sim = 0.0;
  for (int i = 0; i < _dim; i++) {
    cos_sim += other->Get(i) * Get(i);
    other_l2 += other->Get(i) * other->Get(i);
    vec_l2 += Get(i) * Get(i);
  }
  cos_sim /= (std::sqrt(other_l2) * std::sqrt(vec_l2));
  return cos_sim;
}

};
};

#include <google/protobuf/message.h>
#include <google/protobuf/util/time_util.h>

#include <format>
#include <iostream>
#include <string>
// #include <rpc/proto/rpc.pb.h>
// #include <sigmap/sigmap.pb.h>
#include <apps/cossim/proto/cossim.pb.h>

int main(int argc, char **argv) {
  printf("Hi\n");
  std::string f = std::format("num args %d", argc);
  printf("Hi 2: %s\n", f.c_str());
  google::protobuf::Timestamp ts;
  ts.set_seconds(argc);
  ts.set_nanos(argc);
  std::string out;
  google::protobuf::Message &m = ts;
  m.SerializeToString(&out);
  Vector v;
  CosSimReq r;
  r.set_id(112);
  r.set_allocated_inputvec(&v);
  google::protobuf::Message &m2 = r;
  m2.SerializeToString(&out);
  //  Taddr addr;
  //  Blob b;
  return 0;
}

#include <iostream>
#include <format>
#include <string>
//#include <util/log/log.h>

int main(int argc, char **argv) {
  printf("Hi\n");
  std::string f = std::format("num args %d", argc);
  printf("Hi 2: %s\n", f.c_str());
  return 0;
}

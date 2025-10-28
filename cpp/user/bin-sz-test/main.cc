#include <iostream>
#include <util/log/log.h>

int main() {
  printf("Hi\n");
  log(ALWAYS, "Hello!");
  return 0;
}

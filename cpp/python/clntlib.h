#pragma once

#include <sys/stat.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <stdio.h>
#include <stdlib.h>
#include <stdarg.h>
#include <dlfcn.h>
#include <dirent.h>
#include <string.h>
#include <unistd.h>
#include <errno.h>
#include <stdint.h>

#include <iostream>
#include <memory>
#include <expected>

extern "C" {
  void init_socket();
  void started();
  void exited();
  void stat_stub(char* path);
}

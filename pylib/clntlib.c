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

const char* apiSocketEnvVar = "SIGMA_PYAPI_FD";
int apiSfd = 0; 

void init_socket() 
{
  const char* sfd_str = getenv(apiSocketEnvVar);
  if (sfd_str == NULL) {
    exit(-1);
  }
  apiSfd = atoi(sfd_str);
}

void started()
{
  char response[2];
  write(apiSfd, "api/started\n", 12);
  read(apiSfd, response, 1);
  while (response[0] != 'd') {
    read(apiSfd, response, 1);
  }
}

void exited()
{
  char response[2];
  write(apiSfd, "api/exited\n", 11);
  read(apiSfd, response, 1);
  while (response[0] != 'd') {
    read(apiSfd, response, 1);
  }
}

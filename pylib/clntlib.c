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

const char* api_socket_env_var = "SIGMA_PYAPI_FD";
int api_sfd = 0; 

const char* spproxy_env_var = "SIGMA_SPPROXY_FD";
int spproxy_sfd = 0;

uint64_t seqno;

/************
 * Proc API *
 ************/

void init_socket() 
{
  const char* sfd_str = getenv(api_socket_env_var);
  if (sfd_str == NULL) {
    exit(-1);
  }
  api_sfd = atoi(sfd_str);

  const char* spproxy_sfd_str = getenv(spproxy_env_var);
  if (spproxy_sfd_str == NULL) {
    exit(-1);
  }
  spproxy_sfd = atoi(spproxy_sfd_str);

  seqno = 0;
}

void started()
{
  char response[2];
  write(api_sfd, "api/started\n", 12);
  read(api_sfd, response, 1);
  while (response[0] != 'd') {
    read(api_sfd, response, 1);
  }
}

void exited()
{
  char response[2];
  write(api_sfd, "api/exited\n", 11);
  read(api_sfd, response, 1);
  while (response[0] != 'd') {
    read(api_sfd, response, 1);
  }
}

/***********
 * SPProxy *
 ***********/

void write_raw_buffer(const char* buf) {
  int buf_len = strlen(buf);
  printf("Writing %s to SFD %d\n", buf, spproxy_sfd);
  write(spproxy_sfd, buf, buf_len);
}

void write_seqno(uint64_t seqno) {
  // Convert to little Endian
  uint8_t bytes[8];
  for (int i = 0; i < 8; i++) {
    bytes[i] = (seqno >> (i * 8)) & 0xFF;
  }

  write(spproxy_sfd, bytes, 8);
}

void write_frame(const char* frame, uint64_t frame_len) {
  // Write frame_len + 4
  uint8_t bytes[4];
  for (int i = 0; i < 4; i++) {
    bytes[i] = ((frame_len + 4) >> (i * 8)) & 0xFF;
  }
  write(spproxy_sfd, bytes, 4);

  write_raw_buffer(frame);
}

void write_frames(const char** frames, uint32_t num_frames, const uint64_t* frame_lens) {
  // Write num_frames
  uint8_t bytes[4];
  for (int i = 0; i < 4; i++) {
    bytes[i] = ((num_frames + 4) >> (i * 8)) & 0xFF;
  }
  write(spproxy_sfd, bytes, 4);

  for (uint64_t i = 0; i < num_frames; i++) {
    write_frame(frames[i], frame_lens[i]);
  }
}

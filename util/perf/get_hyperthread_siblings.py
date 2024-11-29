#!/usr/bin/python3

def get_processor(info):
  lines = info.split("\n")
  for line in lines:
    if "processor" in line:
      return line.split(" ")[-1]

def get_coreid(info):
  lines = info.split("\n")
  for line in lines:
    if "core id" in line:
      return line.split(" ")[-1]

def main():
  with open("/proc/cpuinfo", "r") as f:
    info = f.read()
  cores = [ i for i in info.split("\n\n") if len(i) > 0 ]
  processors = [ get_processor(core) for core in cores ]
  coreids = [ get_coreid(core) for core in cores ]
  duplicates = [ processors[i] for i in range(len(processors)) if coreids[i] in coreids[:i] ]
  print(*duplicates)

if __name__ == "__main__":
  main()

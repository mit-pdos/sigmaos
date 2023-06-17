#!/usr/bin/python3
import os
import subprocess
import argparse
import yaml
import shlex
import numpy as np
from datetime import datetime

def run_process_get_output(command):
  process = subprocess.Popen(command, stdout=subprocess.PIPE) 
  return str(process.communicate()[0]).replace('\\n', '\n')[2:-1]

def clean_date_string(s):
  # Trim monotonic clock suffix
  s = s[:s.index(" m=+")]
  # Round to microseconds
  di = s.index(".")
  nsi = s.index(" ", di + 1)
  return s[:di + 1] + s[di + 1:di + 7] + s[nsi:]

'''
Parsing strings of the form:

Jun 14 12:40:58 node0.test-thumb-util.ulambda-pg0.wisc.cloudlab.us kubelet[164755]: I0614 12:40:58.105976  164755 pod_startup_latency_tracker.go:102] "Observed pod startup duration" pod="default/thumbnail-benchrealm1-tp8gd" podStartSLOduration=1.849267303 podCreationTimestamp="2023-06-14 12:40:56 -0500 CDT" firstStartedPulling="2023-06-14 12:40:56.799351441 -0500 CDT m=+812.079319978" lastFinishedPulling="2023-06-14 12:40:57.056010141 -0500 CDT m=+812.335978677" observedRunningTime="2023-06-14 12:40:58.099206616 -0500 CDT m=+813.379175152" watchObservedRunningTime="2023-06-14 12:40:58.105926002 -0500 CDT m=+813.385894538"
'''
def parse_pod_stats(s):

  # Ignore everything before fields
  s = s[s.index("pod="):]

  lexer = shlex.shlex(s)
  lexer.whitespace_split = True
  lexer.whitespace += '='
  tokens = list(lexer)

  stats = {}
  for i in range(0, len(tokens), 2):
    key = tokens[i].strip()
    # Remove quotes
    value = tokens[i+1].strip('"')
    stats[key] = value
  
  stats["pod"] = stats["pod"][stats["pod"].index("/") + 1:]
  stats["podStartSLOduration"] = float(stats["podStartSLOduration"])
  stats["podCreationTimestamp"] = datetime.strptime(stats["podCreationTimestamp"], "%Y-%m-%d %H:%M:%S %z %Z")
  stats["firstStartedPulling"] = datetime.strptime(clean_date_string(stats["firstStartedPulling"]), "%Y-%m-%d %H:%M:%S.%f %z %Z")
  stats["lastFinishedPulling"] = datetime.strptime(clean_date_string(stats["lastFinishedPulling"]), "%Y-%m-%d %H:%M:%S.%f %z %Z")
  stats["observedRunningTime"] = datetime.strptime(clean_date_string(stats["observedRunningTime"]), "%Y-%m-%d %H:%M:%S.%f %z %Z")
  stats["watchObservedRunningTime"] = datetime.strptime(clean_date_string(stats["watchObservedRunningTime"]), "%Y-%m-%d %H:%M:%S.%f %z %Z")

  return stats

def parse_kubelet_log(log, pod_names):
  pod_data = [ l for l in log.split("\n") if "Observed pod startup duration" in l ]
  pod_stats = [ parse_pod_stats(p) for p in pod_data ]
  # Filter pods from other runs.
  pod_stats = [ ps for ps in pod_stats if ps["pod"] in pod_names ]
  return pod_stats

def start_time_stats(depname):
  get_pods_out = run_process_get_output(["kubectl", "get", "pods", "--all-namespaces"])
  pod_names = [ n for n in get_pods_out.split() if depname in n ]

  kubelet_log = run_process_get_output(["sudo", "journalctl", "-xeu", "kubelet"])
  pod_stats = parse_kubelet_log(kubelet_log, set(pod_names))
  
  pod_startup_times = [ (s["observedRunningTime"] - s["lastFinishedPulling"]).total_seconds() for s in pod_stats ]
  print("=== Pod startup time\n\tmedian:{}, mean:{}, std:{}".format(np.median(pod_startup_times), np.mean(pod_startup_times), np.std(pod_startup_times))

  diff_pod_scheduled_times = []
  for i in range(1, len(pod_stats)):
    diff = (pod_stats[i]["firstStartedPulling"] - pod_stats[i - 1]["firstStartedPulling"]).total_seconds()
    diff_pod_scheduled_times.append(diff)
  print("=== Time between scheduling events\n\tmedian:{}, mean:{}, std:{}".format(np.median(diff_pod_scheduled_times), np.mean(diff_pod_scheduled_times), np.std(diff_pod_scheduled_times))
 
if __name__ == "__main__":                                                    
  parser = argparse.ArgumentParser()                                        
  parser.add_argument("--depname", type=str, required=True)
                                     
  args = vars(parser.parse_args())
                                     
  start_time_stats(**args) 

#!/usr/bin/python3
import os
import subprocess
import argparse
import yaml
import shlex
import numpy as np
from dateutil import parser as dateparser

def run_process_get_output(command):
  process = subprocess.Popen(command, stdout=subprocess.PIPE) 
  return str(process.communicate()[0]).replace('\\n', '\n')[2:-1]

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
  
  stats["pod"] = stats["pod"].lstrip("default/")
  stats["podStartSLOduration"] = float(stats["podStartSLOduration"])
  stats["podCreationTimestamp"] = dateparser.parse(stats["podCreationTimestamp"])
  stats["firstStartedPulling"] = dateparser.parse(stats["firstStartedPulling"])
  stats["lastFinishedPulling"] = dateparser.parse(stats["lastFinishedPulling"])
  stats["observedRunningTime"] = dateparser.parse(stats["observedRunningTime"])
  stats["watchObservedRunningTime"] = dateparser.parse(stats["watchObservedRunningTime"])

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
  print(pod_stats)
#
#
#
#  pod_details = [ yaml.load(run_process_get_output(["kubectl", "get", "pods", pn, "-o", "yaml"])) for pn in pod_names ]
#  pod_transitions = [ pd["status"]["conditions"] for pd in pod_details ]
#  pod_scheduled_time_strs = [ [ t["lastTransitionTime"] for t in pts if t["type"] == "PodScheduled" ][0] for pts in pod_transitions ]
#  pod_ready_time_strs = [ [ t["lastTransitionTime"] for t in pts if t["type"] == "Ready" ][0] for pts in pod_transitions ]
#  pod_scheduled_times = [ dateparser.parse(s) for s in pod_scheduled_time_strs ]
#  pod_ready_times = [ dateparser.parse(s) for s in pod_ready_time_strs ]
#  pod_startup_times = [ (pod_ready_times[i] - pod_scheduled_times[i]).total_seconds() for i in range(len(pod_ready_times)) ]
#  print("Mean pod startup time:", np.mean(pod_startup_times))
#  print("Std dev pod startup time:", np.std(pod_startup_times))
#  print(np.mean(pod_startup_times))
#  # Calculate mean scheduling latency
#  sorted_pod_scheduled_times = sorted(pod_scheduled_times)
#  diff_pod_scheduled_times = []
#  for i in range(1, len(sorted_pod_scheduled_times)):
#    diff = (sorted_pod_scheduled_times[i] - sorted_pod_scheduled_times[i - 1]).total_seconds()
#    diff_pod_scheduled_times.append(diff)
#  print("Diff sched events:", diff_pod_scheduled_times)
#  print("Mean latency between scheduling events:", np.mean(diff_pod_scheduled_times))
#  print("Std dev between scheduling events:", np.std(diff_pod_scheduled_times))
     
if __name__ == "__main__":                                                    
  parser = argparse.ArgumentParser()                                        
  parser.add_argument("--depname", type=str, required=True)
                                     
  args = vars(parser.parse_args())
                                     
  start_time_stats(**args) 

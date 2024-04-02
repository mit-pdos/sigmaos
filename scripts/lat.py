#!/usr/bin/env python

# scrape a log with NET_LAT

import re
import sys
import time
from datetime import datetime

f = open(sys.argv[1], "r") 
lines = f.readlines()
reqs = {}
reps = {}
lsreq = {}
lsrep = {}

k0 = ("a2610d4aa1325fa9","11")
for l in lines:
    sid = re.search(r'sid ([\da-fA-F]+)', l)
    seq = re.search(r'seq ([\d]+)', l)
    t = re.search(r'NET_LAT ([\w]+)', l)
    s = re.search(r'^([\d:.]+)', l)
    m = re.search(r'type:(\w)', l)
    if sid != None:
        d = datetime.strptime(s.group(1), '%H:%M:%S.%f')
        key =(sid.group(1),seq.group(1))
        if m == None:
            print("parse error", l)
            continue
        if m.groups(1)[0] == 'T':
            if not key in reqs:
                reqs[key] = {t.group(1): d}
                lsreq[key] = {t.group(1): l}
            else:
                reqs[key][t.group(1)] = d
                lsreq[key][t.group(1)] = l

def printlines(ls):
 for l in ls:
     sys.stdout.write(ls[l])
                
for k in lsreq:
    # ignore reqs that don't have a corresponding flush,
    # which are from test program
    if "Flush" in reqs[k]:
        # closing a connection may result in no readcall for flush
        if not "ReadCall" in reqs[k]:
            continue
        d0 = reqs[k]['Flush']
        d1 = reqs[k]['ReadCall']
        diff = d1-d0
        if d1 > d0 and diff.microseconds > 1000:
            print("== long net latency for", k, diff.microseconds)
            printlines(lsreq[k])

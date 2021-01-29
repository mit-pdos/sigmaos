#!/usr/bin/env python3

#
# word-count in map-reduce style
# python3.8 ./wc.py input/pg-*.txt
#

import ulambda
import sys

nreduce = 10

#
# wc map function, to be executed as a lambda.
#
def map(filename):
    out = [ [] for i in range(0, nreduce) ]
    # XXX should read input data from S3/9P.
    with open(filename, "r") as f:
        v = f.read().split()
        for w in v:
            h = hash(w)
            out[h % nreduce].append(w)
    # XXX should write each hash bucket to a separate
    #     9P / S3 file, and return the file names.
    return out

#
# wc reduce function. bucket is a list of words.
#
def reduce(bucket):
    d = { }
    for w in bucket:
        d[w] = d.get(w, 0) + 1
    return [ [ w, d[w] ] for w in d.keys() ]

#
# start the map lambdas.
#
jobs = [ ]
for infile in sys.argv[1:]:
    job = ulambda.run(map, [ infile ])
    jobs.append(job)

#
# wait for all the maps to finish, and
# gather their output.
#
buckets = [ [] for i in range(0, nreduce) ]
for job in jobs:
    bv = job.wait()
    for i in range(0, nreduce):
        buckets[i] += bv[i]

#
# start the reduce lambdas.
#
jobs = [ ]
for i in range(0, nreduce):
    job = ulambda.run(reduce, [ buckets[i] ])
    jobs.append(job)

#
# wait for reduce lambda then print the results.
#
for job in jobs:
    v = job.wait()
    for [ w, n ] in v:
        print("%s %d" % (w, n))

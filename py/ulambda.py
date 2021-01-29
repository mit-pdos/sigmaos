#
# ask ulambda to schedule Python jobs.
#

import secrets
import dill
import os
import sys
import time

#
# a scheduled computation.
#
class Job:

    #
    # ask submit to ask the ulambda scheduler to fire
    # up a python that runs fn(args).
    #
    def __init__(self, fn, args):
        self.id = secrets.randbits(30)

        # if any of the arguments is a Job, tell the
        # scheduler to wait for it.
        # XXX should do a deep search.
        nargs = [ ]
        wait_ids = [ ]
        for a in args:
            if isinstance(a, Job):
                wait_ids.append(a.id)
                nargs.append("XYZZY-" + str(a.id)) # XXX
            else:
                nargs.append(a)
        
        # pickle the function and arguments, and put
        # in a file where the scheduler can find it.
        # XXX all these files should be in 9P or S3.
        picklefile = "/tmp/fn-" + str(self.id)
        f = open(picklefile, "wb")
        dill.dump([ fn, nargs ], f, recurse=True)
        f.close()

        self.outfile = "/tmp/out-" + str(self.id)
        self.donefile = "/tmp/done-" + str(self.id)
        
        # Python script that runs the pickled function
        # and pickles its return value into a file where
        # dependent lambdas can find it.
        cmdfile = "/tmp/cmd-" + str(self.id)
        f = open(cmdfile, "w")
        # f.write("#!/usr/bin/env python3\n\n")
        f.write("#!/opt/local/bin/python3.8\n\n")
        f.write("import dill\n")
        f.write("import sys\n")
        f.write("import os\n")
        f.write("import types\n")
        f.write("sys.stderr.write('this is %s\\n')\n" % (self.id))
        f.write("f = open('%s', 'rb')\n" % (picklefile))
        f.write("g = dill.load(f)\n")
        f.write("f.close()\n")
        f.write("fn = g[0]\n")
        f.write("# read arguments from finished jobs we depend on\n")
        f.write("args = []\n")
        f.write("for a in g[1]:\n")
        f.write("  if isinstance(a, str) and 'XYZZY-' in a:\n")
        f.write("    with open('/tmp/out-' + a[6:], 'rb') as f:\n")
        f.write("      args.append(dill.load(f))\n")
        f.write("  else:\n")
        f.write("    args.append(a)\n")
        f.write("x = fn(*args)\n")
        f.write("# write output where dependent jobs can find it\n")
        f.write("with open('%s', 'wb') as f:\n" % (self.outfile))
        f.write("  dill.dump(x, f)\n")
        f.write("# tell job.wait() we're done\n")
        f.write("open('%s', 'w').close()\n" % (self.donefile))
        f.write("# tell schedd we're done\n")
        f.write("os.system('bin/util exit %s')\n" % (self.id))
        f.close()
        os.system("chmod ogu+x %s" % (cmdfile))

        cmd = "echo '%s,%s,,[],[],%s' | ../bin/submit > /dev/null" % (self.id, cmdfile, wait_ids)
        os.system(cmd)

    #
    # wait for completion, return result.
    #
    def wait(self):
        # XXX is there a way to ask the scheduler whether
        # a lambda has finished?
        while True:
            if os.access(self.donefile, os.R_OK) == True:
                break
            time.sleep(1)
        with open(self.outfile, "rb") as f:
            return dill.load(f)
            

def run(fn, args):
    th = Job(fn, args)
    return th

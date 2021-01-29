#
# ask ulambda to schedule Python jobs.
#

import secrets
import marshal
import os
import sys

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
        marshal.dump([ fn.__code__, nargs ], f);
        f.close()

        outfile = "/tmp/out-" + str(self.id)
        
        # Python script that runs the pickled function
        # and pickles its return value into a file where
        # dependent lambdas can find it.
        cmdfile = "/tmp/cmd-" + str(self.id)
        f = open(cmdfile, "w")
        f.write("#!/usr/bin/env python3\n\n")
        f.write("import marshal\n")
        f.write("import sys\n")
        f.write("import types\n")
        f.write("sys.stderr.write('this is %s\\n')\n" % (self.id))
        f.write("f = open('%s', 'rb')\n" % (picklefile))
        f.write("g = marshal.load(f)\n")
        f.write("f.close()\n")
        f.write("fn = types.FunctionType(g[0], {}, 'fff')\n")
        f.write("# read arguments from finished jobs we depend on\n")
        f.write("args = []\n")
        f.write("for a in g[1]:\n")
        f.write("  if isinstance(a, str) and 'XYZZY-' in a:\n")
        f.write("    with open('/tmp/out-' + a[5:], 'rb') as f:\n")
        f.write("      args.append(marshal.load(f))\n")
        f.write("  else:\n")
        f.write("    args.append(a)\n")
        f.write("x = fn(args)\n")
        f.write("# write output where dependent jobs can find it\n")
        f.write("with open('%s', 'wb') as f:\n" % (outfile))
        f.write("  marshal.dump(x, f)\n")
        f.close()
        os.system("chmod ogu+x %s" % (cmdfile))

        cmd = "echo '%s,%s,,[],[],%s' | ../bin/submit" % (self.id, cmdfile, wait_ids)
        sys.stderr.write("cmd: %s\n" % (cmd))
        os.system(cmd)

def run(fn, args):
    th = Job(fn, args)
    return th

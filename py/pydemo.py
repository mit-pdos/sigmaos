import ulambda

#
# a function to be run by the ulambda scheduler.
#
def f1(x):
    return 1000

#
# another.
#
def f2(x):
    return 2000

#
# ask ulambda to run f1(11). don't wait (yet) for
# the result. v1 is a handle to the eventual result.
#
v1 = ulambda.run(f1, [ 11 ])

#
# ask ulambda to run f2(<the value f1 will return>).
#
v2 = ulambda.run(f2, [ v1 ])

#
# print v2's value, after waiting for it to finish.
#
print(v2.wait())



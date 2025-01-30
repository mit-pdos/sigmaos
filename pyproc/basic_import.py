import splib
import shlex

splib.started()
print(shlex.join(["Hello", "World"]))
splib.exited()

import splib
import shlex

splib.Started()
print(shlex.join(["Hello", "World"]))
splib.Exited()

import splib
import custom_lib
from nested_import import utils

splib.Started()
custom_lib.sayHi()
print("The square of 5 is:", utils.square(5))
splib.Exited()

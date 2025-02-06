import splib
import sys
sys.path.append('/~~/pyproc')
from neighbor_import import custom_lib

splib.started()
custom_lib.sayHi()
splib.exited()

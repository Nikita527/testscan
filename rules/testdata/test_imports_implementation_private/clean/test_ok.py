from mymodule import helper
import _thread
import _ast
from pkg import __version__

def test_ok():
    assert helper() == 1

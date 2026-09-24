from mymodule import _helper

def test_uses_private():
    assert _helper() == 1

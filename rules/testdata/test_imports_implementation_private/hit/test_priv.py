from mymodule import _helper, _normalize, _emit

def test_uses_private():
    assert _helper() == 1

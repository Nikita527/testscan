from mymodule import _FOO_BAR, _HTTP_STATUS

def test_ok():
    assert _FOO_BAR == 1
    assert _HTTP_STATUS == 200

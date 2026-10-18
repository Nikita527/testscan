import time


def test_fixed_sleep_then_return():
    time.sleep(1)
    assert ready()
    return

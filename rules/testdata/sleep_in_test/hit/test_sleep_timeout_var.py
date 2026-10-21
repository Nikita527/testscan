import time


def test_sleep_with_timeout_variable():
    timeout = 5
    time.sleep(1)
    assert ready()
    assert timeout > 0

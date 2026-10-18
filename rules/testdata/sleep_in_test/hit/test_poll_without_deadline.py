import time


def test_poll_loop_without_deadline():
    while not ready():
        time.sleep(0.1)
        if tries() > 10:
            break
    assert ready()

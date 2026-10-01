from datetime import datetime


def test_wall_clock():
    now = datetime.now()
    assert now is not None

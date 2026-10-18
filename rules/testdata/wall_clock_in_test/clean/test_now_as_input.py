from datetime import datetime


def test_now_passed_as_input():
    stamp = datetime.now()
    result = build(created_at=stamp)
    assert result.ok

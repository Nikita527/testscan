def sanitize(x):
    return type("R", (), {"fp": x})()


def test_determinism_on_call_attr():
    assert sanitize(1).fp == sanitize(1).fp

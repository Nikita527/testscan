def plan_delete_cascade(x):
    return x


def build():
    return 1


def test_determinism():
    assert plan_delete_cascade(build()) == plan_delete_cascade(build())

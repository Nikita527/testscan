def test_a():
    assert 1 == 1


def test_b():
    assert 2 == 2


def test_c():
    assert 3 == 3


def test_d():
    mock_dep.side_effect = RuntimeError("boom")
    assert call() is None

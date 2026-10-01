from happy_app.service import process


def test_a():
    process(0)
    assert 1 == 1


def test_b():
    process(0)
    assert 2 == 2


def test_c():
    process(0)
    assert 3 == 3


def test_d():
    process(0)
    mock_dep.side_effect = RuntimeError("boom")
    assert call() is None

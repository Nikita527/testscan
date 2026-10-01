from happy_app.service import process


def test_a():
    assert process(1) == 1


def test_b():
    assert process(2) == 2


def test_c():
    assert process(3) == 3


def test_d():
    assert process(4) == 4

import pytest


def test_a():
    assert 1 == 1


def test_b():
    assert 2 == 2


def test_c():
    assert 3 == 3


def test_d():
    with pytest.raises(ValueError):
        raise ValueError("x")

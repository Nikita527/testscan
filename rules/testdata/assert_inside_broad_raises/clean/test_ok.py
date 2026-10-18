import pytest


def test_specific_raises():
    with pytest.raises(PermissionError):
        connect_without_key()


def test_raises_then_assert_outside():
    with pytest.raises(ValueError):
        boom()
    assert True

import pytest


def test_raises_value_error():
    with pytest.raises(ValueError):
        boom()

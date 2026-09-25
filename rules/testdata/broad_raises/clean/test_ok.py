import pytest


def test_ok():
    with pytest.raises(ValueError, match="boom"):
        raise ValueError("boom")

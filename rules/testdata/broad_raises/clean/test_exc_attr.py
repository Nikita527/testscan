import pytest


def test_specific_with_attr():
    with pytest.raises(ValueError) as exc_info:
        raise ValueError("boom")
    assert exc_info.value.args[0] == "boom"

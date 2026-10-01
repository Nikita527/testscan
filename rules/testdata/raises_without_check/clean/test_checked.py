import pytest


def test_raises_with_match():
    with pytest.raises(ValueError, match="bad"):
        boom()


def test_raises_with_code():
    with pytest.raises(DomainError) as exc_info:
        boom()
    assert exc_info.value.code == "X"

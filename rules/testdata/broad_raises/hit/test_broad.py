import pytest


def test_broad():
    with pytest.raises(Exception):
        raise ValueError("boom")

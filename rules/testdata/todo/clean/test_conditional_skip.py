import pytest


def test_skip_when_missing():
    pytest.skip(f"Reference structure file not present: {path}")

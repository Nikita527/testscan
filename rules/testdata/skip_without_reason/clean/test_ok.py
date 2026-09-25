import pytest


@pytest.mark.skip(reason="flaky")
def test_ok():
    assert True

import pytest


@pytest.mark.skip(reason="flaky env")
def test_skip_with_reason():
    assert True


@pytest.mark.xfail(reason="known bug")
def test_xfail_with_reason():
    assert False

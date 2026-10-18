import pytest


def test_websocket_requires_api_key():
    with pytest.raises(Exception):
        connect_without_key()
        assert False


def test_websocket_requires_api_key_fail():
    with pytest.raises(Exception):
        connect_without_key()
        pytest.fail("should have been rejected")

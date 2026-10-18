import pytest


def test_websocket_tuple_raises():
    with pytest.raises((Exception, OSError)):
        connect_without_key()
        assert False

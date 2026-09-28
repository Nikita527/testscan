from unittest.mock import MagicMock


def test_mock_helper():
    m = MagicMock(return_value=1)
    assert helper(m) == 1

from unittest.mock import Mock


def test_mock_called_is_real():
    mock = Mock()
    mock()
    assert mock.called

from unittest.mock import MagicMock


def test_mock_called_by_itself():
    notifier = MagicMock()
    notifier.send("hello")
    notifier.send.assert_called_once_with("hello")

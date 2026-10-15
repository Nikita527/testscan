from unittest.mock import patch


def test_comment_mentions_lock():
    # Prefer monkeypatch; SUT uses select_for_update in production
    with patch("app.services.notify") as mock_notify:
        notify()
        mock_notify.assert_called_once()

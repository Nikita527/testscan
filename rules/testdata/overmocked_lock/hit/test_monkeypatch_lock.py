from unittest.mock import MagicMock


def test_race_monkeypatch_lock(monkeypatch):
    mock_sfu = MagicMock()
    monkeypatch.setattr("app.models.Order.objects.select_for_update", mock_sfu)
    process_order(1)
    mock_sfu.assert_called_once()

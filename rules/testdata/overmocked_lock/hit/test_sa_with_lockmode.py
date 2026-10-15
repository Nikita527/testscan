from unittest.mock import patch

@patch("app.services.orders.query.with_lockmode")
def test_race_uses_legacy_lock(mock_wlm):
    process_order(1)
    mock_wlm.assert_called_once()

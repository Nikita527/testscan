from unittest.mock import patch

@patch("app.services.orders.select.with_for_update")
def test_race_uses_sa_lock(mock_wfu):
    process_order(1)
    mock_wfu.assert_called_once()

from unittest.mock import patch

@patch("app.models.Order.objects.select_for_update")
def test_race_uses_lock(mock_sfu):
    process_order(1)
    mock_sfu.assert_called_once()

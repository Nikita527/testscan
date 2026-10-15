from unittest.mock import patch


@patch("app.models.Order.objects.select_for_update_of")
def test_race_uses_sfu_of(mock_sfu):
    process_order(1)
    mock_sfu.assert_called_once()

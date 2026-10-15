from unittest.mock import MagicMock


def test_race_attr_replace_lock():
    mock_sfu = MagicMock()
    Order.objects.select_for_update = mock_sfu
    process_order(1)
    mock_sfu.assert_called_once()

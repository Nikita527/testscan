from unittest.mock import patch


def test_race_multiline_with_patch():
    with patch(
        "app.models.Order.objects.select_for_update"
    ) as mock_sfu:
        process_order(1)
        mock_sfu.assert_called_once()

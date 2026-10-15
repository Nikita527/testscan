from unittest.mock import patch

@patch("app.models.Order.objects.select_for_update")
def test_patched_no_mock_assert(mock_sfu):
    process_order(1)

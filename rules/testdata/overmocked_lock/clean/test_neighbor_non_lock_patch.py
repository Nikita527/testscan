from unittest.mock import patch

# Non-test helper above a mock-only test: old lookback window would false-positive.


@patch("app.models.Order.objects.select_for_update")
def _setup_lock_patch(mock_sfu):
    pass


@patch("app.services.helper")
def test_other_mock_only(mock_helper):
    helper()
    mock_helper.assert_called_once()

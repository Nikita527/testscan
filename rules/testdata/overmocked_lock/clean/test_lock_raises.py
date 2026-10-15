import pytest
from unittest.mock import patch

@patch("app.models.Order.objects.select_for_update")
def test_lock_conflict(mock_sfu):
    mock_sfu.side_effect = OperationalError("locked")
    with pytest.raises(OperationalError):
        process_order(1)
    mock_sfu.assert_called()

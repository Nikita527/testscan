from unittest.mock import patch


def test_setattr_restore_lock():
    original = Order.objects.select_for_update
    setattr(Order.objects, "select_for_update", original)
    with patch("app.services.notify") as mock_notify:
        notify()
        mock_notify.assert_called_once()

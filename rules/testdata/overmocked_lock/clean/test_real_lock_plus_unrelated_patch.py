from unittest.mock import patch


def test_real_lock_and_unrelated_patch():
    # Real lock call must not combine with an unrelated patch into a hit.
    qs = Order.objects.select_for_update().filter(id=1)
    with patch("app.services.notify") as mock_notify:
        notify(qs)
        mock_notify.assert_called_once()

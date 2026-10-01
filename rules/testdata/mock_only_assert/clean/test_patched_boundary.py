from unittest.mock import patch

from df_app.service import process


def test_patched_boundary_called():
    with patch("df_app.service.send_mail") as send:
        process(1)
    send.assert_called_once()

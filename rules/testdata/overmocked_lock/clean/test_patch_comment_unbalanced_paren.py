from unittest.mock import patch


def test_patch_comment_unbalanced_paren():
    with patch(  # TODO (revisit later
        "app.services.notify"
    ) as mock_notify:
        Order.objects.select_for_update().filter(id=1)
        mock_notify.assert_called_once()

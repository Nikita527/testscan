from unittest.mock import MagicMock

from df_app.service import place_order


def test_place_order_saves_user():
    repo = MagicMock()
    place_order("alice", repo)
    repo.save.assert_called_once_with("alice")


def test_mock_attached_to_object():
    repo = MagicMock()
    holder = Holder()
    holder.repo = repo
    run(holder)
    repo.save.assert_called()

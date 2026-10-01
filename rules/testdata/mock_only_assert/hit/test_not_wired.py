from unittest.mock import MagicMock

from df_app.service import process


def test_mock_never_reaches_sut():
    audit = MagicMock()
    process(1)
    audit.record.assert_called_once()

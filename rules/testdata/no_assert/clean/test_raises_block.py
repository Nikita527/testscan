import pytest

from df_app.service import process


def test_negative_rejected():
    with pytest.raises(ValueError):
        process(-1)

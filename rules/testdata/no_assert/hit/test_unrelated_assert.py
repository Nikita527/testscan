import pytest

from df_app.service import process


def test_assert_unrelated_literal():
    result = process(3)
    expected = 4
    assert expected == 4


def test_assert_other_fixture(settings):
    process(2)
    assert settings.DEBUG is False


def test_helper_unrelated():
    process(1)
    assert_valid_config({"a": 1})


def test_raises_elsewhere():
    process(1)
    with pytest.raises(ZeroDivisionError):
        1 / 0

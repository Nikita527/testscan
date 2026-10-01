from df_app.service import process


def test_derived_from_mock_only(m):
    m.return_value = 42
    value = m()
    process(1)
    assert value == 42


def test_return_value_attr():
    m.return_value = 7
    assert m.return_value == 7

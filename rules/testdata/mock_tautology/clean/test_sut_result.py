from df_app.service import process


def test_sut_receives_mock():
    m.return_value = 42
    result = process(m)
    assert result == m.return_value


def test_mock_state_not_tautology():
    m.return_value = 1
    process(m)
    assert m.call_count == 1

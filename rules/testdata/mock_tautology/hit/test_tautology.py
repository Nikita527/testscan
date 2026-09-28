def test_tautology():
    m.return_value = 42
    assert m() == 42

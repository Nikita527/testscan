def test_tautology():
    m.return_value = 42
    assert sut() == 42

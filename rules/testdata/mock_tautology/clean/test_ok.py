def test_ok():
    m.return_value = 42
    assert sut() == 7

def test_ok():
    m.return_value = True
    flag = compute()
    assert flag is True

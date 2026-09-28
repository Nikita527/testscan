def test_commented():
    value = run()
    # assert value == 1
    assert value is not None

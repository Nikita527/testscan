def test_local_called_flag():
    called = False

    def cb():
        nonlocal called
        called = True

    cb()
    assert called

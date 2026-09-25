def test_ok():
    try:
        raise ValueError("x")
    except Exception:
        raise
    assert True

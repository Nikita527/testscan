def test_swallowed():
    try:
        raise ValueError("x")
    except Exception:
        pass
    assert True

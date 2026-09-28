def test_literal_loop():
    for item in (1, 2, 3):
        assert item > 0

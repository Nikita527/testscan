def test_ok():
    items = [1]
    assert items
    for item in items:
        assert item == 1

def test_batch_empty():
    items = collect()
    assert len(items) == 0

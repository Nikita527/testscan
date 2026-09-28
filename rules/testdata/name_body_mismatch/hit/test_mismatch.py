def test_rejects_missing_item():
    result = process(item)
    assert result.is_valid

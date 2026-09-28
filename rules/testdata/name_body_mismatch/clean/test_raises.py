def test_rejects_missing_item():
    with pytest.raises(ValueError):
        process(item)

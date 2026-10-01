def test_rejects_missing_item():
    result = process(item)
    assert result.is_valid


def test_fails_with_bad_payload():
    result = process(payload)
    assert result.count == 3


def test_returns_error_for_empty_name():
    result = create(name="")
    assert result.id == 7

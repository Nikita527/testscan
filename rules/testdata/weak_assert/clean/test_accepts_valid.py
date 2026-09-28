def test_accepts_valid_payload():
    result = validate(payload)
    assert result.is_valid, result.errors

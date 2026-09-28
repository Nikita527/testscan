def test_accepts_valid_input():
    validate_payload(data)


def test_does_not_raise_on_empty():
    process([])

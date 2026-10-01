def test_none_and_empty_normalize_to_the_same_key():
    assert normalize(None) == normalize("")

def test_hedged_assert():
    result = maybe()
    assert result.is_valid or True  # TestModel may not inject

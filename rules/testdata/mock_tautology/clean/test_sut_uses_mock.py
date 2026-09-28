def test_sut_echoes_mock_value_ok():
    """SUT call returning mocked dependency value is not a tautology."""
    m.return_value = None
    assert ensure.execute() is None

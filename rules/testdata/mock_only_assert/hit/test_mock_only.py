def test_mock_only():
    result = compute()
    mock.assert_called()

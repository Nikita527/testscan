def test_secret_detected():
    result = detect(secret)
    assert result.has_secrets

def test_returns_400_without_error_code():
    response = client.post("/items", json={})
    assert response.status_code == 400

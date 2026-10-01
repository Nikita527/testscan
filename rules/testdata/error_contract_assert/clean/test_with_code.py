def test_returns_400_with_error_code():
    response = client.post("/items", json={})
    assert response.status_code == 400
    assert response.data["errors"][0]["code"] == "invalid"

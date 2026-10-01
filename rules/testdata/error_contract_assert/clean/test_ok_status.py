def test_ok_200():
    response = client.get("/items")
    assert response.status_code == 200

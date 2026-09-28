def test_field_checks():
    data = resp.json()
    assert data["id"] == 1
    assert data["name"] == "x"

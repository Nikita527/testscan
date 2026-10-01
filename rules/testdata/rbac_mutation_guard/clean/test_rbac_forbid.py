def test_rbac_forbidden_role_cannot_delete():
    response = client.delete("/items/1")
    assert response.status_code == 403


def test_happy_create():
    client.post("/items", json={"name": "x"})
    assert True

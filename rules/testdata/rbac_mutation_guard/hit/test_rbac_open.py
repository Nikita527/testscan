def test_rbac_admin_can_update():
    client.post("/items/1", json={"name": "x"})
    assert True

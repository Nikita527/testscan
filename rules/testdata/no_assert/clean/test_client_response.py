def test_post_created(client):
    response = client.post("/orders/", {"sku": "a"})
    assert response.status_code == 201


def test_client_side_effect(api_client, db):
    api_client.delete("/orders/1/")
    assert not api_client.session.get("cart")

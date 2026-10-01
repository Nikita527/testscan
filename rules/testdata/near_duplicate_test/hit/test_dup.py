def test_unknown_user_returns_404(client):
    resp = client.get("/users/999")
    assert resp.status_code == 404


def test_unknown_order_returns_404(client):
    resp = client.get("/orders/123")
    assert resp.status_code == 404


def test_unknown_item_returns_404(client):
    resp = client.get("/items/42")
    assert resp.status_code == 404

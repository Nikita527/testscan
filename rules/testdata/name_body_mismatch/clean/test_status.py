def test_rejects_bad_status():
    resp = client.get("/x")
    assert resp.status_code == 404

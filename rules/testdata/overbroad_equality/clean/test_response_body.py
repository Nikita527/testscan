def test_response_data_contract():
    response = client.get("/item")
    assert response.data == {
        "a": 1,
        "b": 2,
        "c": 3,
        "d": 4,
        "e": 5,
        "f": 6,
        "g": 7,
        "h": 8,
        "i": 9,
        "j": 10,
        "k": 11,
        "l": 12,
        "m": 13,
    }


def test_resp_json_contract():
    resp = client.get("/item")
    assert resp.json() == {
        "a": 1,
        "b": 2,
        "c": 3,
        "d": 4,
        "e": 5,
        "f": 6,
        "g": 7,
        "h": 8,
        "i": 9,
        "j": 10,
        "k": 11,
        "l": 12,
        "m": 13,
    }


def test_nested_response_data_index():
    response = client.get("/item")
    assert response.data["results"][0]["fields"] == [
        {"a": 1, "b": 2, "c": 3, "d": 4},
        {"a": 5, "b": 6, "c": 7, "d": 8},
        {"a": 9, "b": 10, "c": 11, "d": 12},
        {"a": 13, "b": 14, "c": 15, "d": 16},
    ]


def test_json_items_subscript():
    resp = client.get("/item")
    assert resp.json()["items"] == [
        {"a": 1, "b": 2, "c": 3, "d": 4},
        {"a": 5, "b": 6, "c": 7, "d": 8},
        {"a": 9, "b": 10, "c": 11, "d": 12},
        {"a": 13, "b": 14, "c": 15, "d": 16},
    ]


def test_values_list_rows():
    assert list(Model.objects.values_list("id", "name")) == [(1, "a"), (2, "b"), (3, "c"), (4, "d"), (5, "e"), (6, "f"), (7, "g"), (8, "h"), (9, "i"), (10, "j"), (11, "k"), (12, "l"), (13, "m")]

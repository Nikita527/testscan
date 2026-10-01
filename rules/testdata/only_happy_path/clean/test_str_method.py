import json

from happy_app.service import process


def test_size_a():
    payload = json.dumps({"a": 1})
    assert len(payload.encode("utf-8")) == 8


def test_size_b():
    payload = json.dumps({"b": 2})
    assert len(payload.encode("utf-8")) == 8


def test_size_c():
    text = "abc"
    assert len(text.encode("utf-8")) == 3


def test_size_d():
    assert len(",".join(["a", "b"])) == 3

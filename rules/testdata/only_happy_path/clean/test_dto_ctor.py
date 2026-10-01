import json

from happy_app.service import Meta


def test_a():
    assert json.dumps({"k": Meta("a").name}) == '{"k": "a"}'


def test_b():
    assert json.dumps({"k": Meta("b").name}) == '{"k": "b"}'


def test_c():
    assert json.dumps([Meta("c").name]) == '["c"]'


def test_d():
    assert json.dumps({"k": Meta("d").name}) == '{"k": "d"}'

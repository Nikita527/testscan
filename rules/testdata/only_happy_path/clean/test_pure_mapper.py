def label_for(code):
    return {"a": "A", "b": "B"}.get(code, code)


def test_label_a():
    assert label_for("a") == "A"


def test_label_b():
    assert label_for("b") == "B"


def test_label_c():
    assert label_for("c") == "c"


def test_label_d():
    assert label_for("d") == "d"

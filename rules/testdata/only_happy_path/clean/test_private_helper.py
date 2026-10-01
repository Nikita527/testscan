from happy_app.service import process


def _rows(n):
    return [{"id": i} for i in range(n)]


def test_one_row():
    rows = _rows(1)
    assert len(rows) == 1


def test_two_rows():
    rows = _rows(2)
    assert len(rows) == 2


def test_three_rows():
    rows = _rows(3)
    assert len(rows) == 3


def test_four_rows():
    rows = _rows(4)
    assert len(rows) == 4

def test_a():
    assert 1 == 1


def test_b():
    assert 2 == 2


def test_c():
    assert 3 == 3


def test_d(caplog):
    run()
    assert "failed" in caplog.text
    assert any(r.levelname == "ERROR" for r in caplog.records)
